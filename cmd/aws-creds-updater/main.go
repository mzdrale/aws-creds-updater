package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	updater "github.com/mzdrale/aws-creds-updater/updater"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

var (
	binName string
	version string
	cfgDir  string
	cfgFile string
)

// Config structures
type Account struct {
	Name    string   `yaml:"name"`
	Aliases []string `yaml:"aliases"`
}

type Config struct {
	Profiles       []Account `yaml:"profiles"`
	AWSSecretsFile string    `yaml:"aws_secrets_file"`
}

// Argument variables
var (
	aPrintVersion bool
)

func init() {

	userHomeDir, err := updater.GetLocalUserHomeDir()
	if err != nil {
		log.Printf("[ERROR] Couldn't determine your homedir: %s\n\n", err)
	}

	cfgDir = path.Join(userHomeDir, ".config/aws-creds-updater")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")    // Look in current directory first
		viper.AddConfigPath(cfgDir) // Then look in user config dir
	}

	// Try to read config
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf(updater.Fatal("\U00002717 Unable to read configuration file: %s\n\n"), err.Error())
		os.Exit(1)
	}

	// Usage
	flag.Usage = func() {
		fmt.Printf("Usage: \n")
		flag.PrintDefaults()
	}

	// Get arguments
	flag.BoolVarP(&aPrintVersion, "version", "V", false, "Print version")

	flag.Parse()
}

// maskCredential shows first 5 and last 5 chars, replacing the middle with *
func maskCredential(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 5 {
		return s + "***"
	}
	first5 := s[:5]
	suffixStart := len(s) - 5
	if suffixStart < 5 {
		suffixStart = 5
	}
	suffix := s[suffixStart:]
	middleLen := len(s) - 5 - len(suffix)
	if middleLen < 1 {
		middleLen = 1
	}
	return first5 + strings.Repeat("*", middleLen) + suffix
}

var (
	credKeyRegex = regexp.MustCompile(`^(aws_(access_key_id|secret_access_key|session_token))\s*=\s*(\S+)$`)
	sectionRegex = regexp.MustCompile(`^\[(.+)\]$`)
	bom          = "\ufeff"
)

// maskCredentialLine masks credential values in a line for display (first 5 and last 5 visible)
func maskCredentialLine(line string) string {
	m := credKeyRegex.FindStringSubmatch(line)
	if len(m) > 0 {
		return m[1] + "=" + maskCredential(m[3])
	}
	return line // section headers etc. shown as-is
}

// normalizeLine trims whitespace and BOM that can appear when pasting from browser
func normalizeLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, bom)
	return strings.TrimSpace(line)
}

// readCredentialLines reads multi-line input. When stdin is a terminal, input is
// read without echo but each line is displayed masked (first 5 and last 5 chars visible).
//
// term.ReadPassword cannot be used here: it keeps canonical (line) mode, and on
// macOS MAX_CANON is 1024. AWS session tokens are often longer, so the kernel
// stops accepting input before the newline arrives and the read hangs.
func readCredentialLines() ([]string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		return readCredentialLinesFromTerminal(fd)
	}
	// Piped input - use scanner
	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := normalizeLine(scanner.Text())
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

var errInterrupted = errors.New("interrupted")

const pasteIdle = 200 * time.Millisecond

// readCredentialLinesFromTerminal reads a pasted credential block in raw,
// non-blocking mode. Line-oriented reads (ReadPassword, Peek after \r, or
// a blocking drain) hang here: macOS Enter is only \r, session tokens are
// longer than the TTY queue, and leftover paste bytes are not a timeout.
func readCredentialLinesFromTerminal(fd int) ([]string, error) {
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer term.Restore(fd, oldState)

	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		return nil, err
	}
	defer unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
	if err := unix.SetNonblock(fd, true); err != nil {
		return nil, err
	}

	var p credPaste
	buf := make([]byte, 4096)
	var idleSince time.Time

	for {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			if feedErr := p.feed(buf[:n]); feedErr != nil {
				return nil, feedErr
			}
			p.echoNew()
			idleSince = time.Time{}
			if p.blank || p.eof {
				p.finishPartial()
				p.echoNew()
				return p.lines, nil
			}
			continue
		}
		if err == unix.EINTR {
			continue
		}
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			if hasCompleteCredentials(p.currentLines()) {
				if idleSince.IsZero() {
					idleSince = time.Now()
				} else if time.Since(idleSince) >= pasteIdle {
					p.finishPartial()
					p.echoNew()
					return p.lines, nil
				}
			}
			time.Sleep(15 * time.Millisecond)
			continue
		}
		if err == nil || err == io.EOF {
			p.finishPartial()
			p.echoNew()
			return p.lines, nil
		}
		return nil, err
	}
}

// credPaste accumulates raw TTY bytes into credential lines.
type credPaste struct {
	line    []byte
	lines   []string
	printed int
	esc     int
	skipLF  bool
	blank   bool
	eof     bool
}

func (p *credPaste) feed(data []byte) error {
	for _, b := range data {
		if err := p.feedByte(b); err != nil {
			return err
		}
	}
	return nil
}

func (p *credPaste) feedByte(b byte) error {
	if p.esc == 1 {
		if b == '[' {
			p.esc = 2
			return nil
		}
		p.esc = 0
	}
	if p.esc == 2 {
		if b >= 0x40 && b <= 0x7E {
			p.esc = 0
		}
		return nil
	}
	if p.skipLF {
		p.skipLF = false
		if b == '\n' {
			return nil
		}
	}
	switch b {
	case 3:
		return errInterrupted
	case 4:
		p.eof = true
		return nil
	case '\r':
		p.endLine()
		p.skipLF = true
		return nil
	case '\n':
		p.endLine()
		return nil
	case 127, '\b':
		if len(p.line) > 0 {
			p.line = p.line[:len(p.line)-1]
		}
		return nil
	case 0x1b:
		p.esc = 1
		return nil
	default:
		if b >= 32 {
			p.line = append(p.line, b)
		}
		return nil
	}
}

func (p *credPaste) endLine() {
	s := normalizeLine(string(p.line))
	p.line = p.line[:0]
	if s == "" {
		p.blank = true
		return
	}
	p.lines = append(p.lines, s)
}

func (p *credPaste) finishPartial() {
	s := normalizeLine(string(p.line))
	p.line = p.line[:0]
	if s != "" {
		p.lines = append(p.lines, s)
	}
}

func (p *credPaste) currentLines() []string {
	out := append([]string(nil), p.lines...)
	if s := normalizeLine(string(p.line)); s != "" {
		out = append(out, s)
	}
	return out
}

func (p *credPaste) echoNew() {
	for p.printed < len(p.lines) {
		fmt.Print(maskCredentialLine(p.lines[p.printed]) + "\r\n")
		p.printed++
	}
}

// hasCompleteCredentials reports whether the pasted block has a profile
// section and all three AWS keys, so we can stop without a blank Enter.
func hasCompleteCredentials(lines []string) bool {
	var hasSection, hasKey, hasSecret, hasToken bool
	for _, line := range lines {
		if sectionRegex.MatchString(line) {
			hasSection = true
		}
		m := credKeyRegex.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		switch m[1] {
		case "aws_access_key_id":
			hasKey = true
		case "aws_secret_access_key":
			hasSecret = true
		case "aws_session_token":
			hasToken = true
		}
	}
	return hasSection && hasKey && hasSecret && hasToken
}

// findAccountNameByAlias finds the account name for a given alias
func findAccountNameByAlias(config Config, alias string) string {
	for _, account := range config.Profiles {
		for _, accountAlias := range account.Aliases {
			if accountAlias == alias {
				return account.Name
			}
		}
	}
	return alias // Return the original alias if no mapping found
}

func main() {

	if aPrintVersion {
		fmt.Printf("\n%v %v\n\n", binName, version)
		fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())
		fmt.Printf("URL: https://github.com/mzdrale/aws-creds-updater\n\n")
		os.Exit(0)
	}

	// Read config data
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		fmt.Printf(updater.Fatal("\U00002717 Unable to parse configuration file: %s\n\n"), err.Error())
		os.Exit(1)
	}

	// Fallback: if unmarshal didn't work, get directly from viper
	if config.AWSSecretsFile == "" {
		config.AWSSecretsFile = viper.GetString("aws_secrets_file")
	}

	fmt.Printf("✓ Loaded config with %d profiles from: %s\n", len(config.Profiles), viper.ConfigFileUsed())

	accounts := updater.Accounts{}
	var currentAlias string
	lineCount := 0
	validInputFound := false

	fmt.Println("Ready to read AWS credentials from stdin (paste your credentials)...")
	fmt.Println("(Credentials shown with first 5 and last 5 chars visible, rest masked)")

	lines, err := readCredentialLines()
	if err != nil {
		fmt.Printf(updater.Fatal("Error reading input: %s\n"), err)
		os.Exit(1)
	}

	// Process lines
	for _, line := range lines {
		lineCount++
		line = normalizeLine(line)

		// Skip lines that look like random text (use regex - more robust than HasPrefix for pasted content with BOM etc.)
		if !sectionRegex.MatchString(line) && !credKeyRegex.MatchString(line) {
			fmt.Printf(updater.Warn("Warning: Ignoring line %d - doesn't match expected AWS credential format\n"), lineCount)
			continue
		}

		// Parse account section header
		m := sectionRegex.FindStringSubmatch(line)

		if len(m) > 0 {
			currentAlias = m[1]
			validInputFound = true
			// Extract account ID from alias (format: 1111111111_Dev-Admin)
			parts := strings.Split(currentAlias, "_")
			if len(parts) > 0 {
				accounts.Id = parts[0]
			}
		}

		m = credKeyRegex.FindStringSubmatch(line)
		key := ""
		value := ""
		if len(m) > 0 {
			key = m[1]
			value = m[3]
			validInputFound = true

			if key == "aws_access_key_id" {
				accounts.AccessKeyId = value
			}

			if key == "aws_secret_access_key" {
				accounts.SecretAccessKey = value
			}

			if key == "aws_session_token" {
				accounts.SessionToken = value
			}

		}
	}

	// Validate that we received valid input
	if !validInputFound {
		fmt.Print(updater.Fatal("\U00002717 No valid AWS credential data found in input.\n"))
		fmt.Println("Expected format:")
		fmt.Println("  [1234567890_Account-Name]")
		fmt.Println("  aws_access_key_id=AKIA...")
		fmt.Println("  aws_secret_access_key=...")
		fmt.Println("  aws_session_token=...")
		os.Exit(1)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("INPUT PROCESSING COMPLETE")
	fmt.Println(strings.Repeat("=", 50))

	// Show what was found
	if currentAlias != "" {
		fmt.Printf("✓ Account section: %s\n", currentAlias)
	}
	if accounts.AccessKeyId != "" {
		fmt.Printf("✓ Access Key ID: %s\n", maskCredential(accounts.AccessKeyId))
	}
	if accounts.SecretAccessKey != "" {
		fmt.Printf("✓ Secret Access Key: %s\n", maskCredential(accounts.SecretAccessKey))
	}
	if accounts.SessionToken != "" {
		fmt.Printf("✓ Session Token: %s\n", maskCredential(accounts.SessionToken))
	}

	// Find the friendly account name using the config
	var accountName string
	if currentAlias != "" {
		accountName = findAccountNameByAlias(config, currentAlias)
		if accountName != currentAlias {
			fmt.Printf("✓ Account mapping: %s → %s\n", currentAlias, accountName)
		} else {
			// No mapping found, use alias as profile name
			accountName = currentAlias
			fmt.Printf(updater.Warn("⚠ No config mapping found for '%s', using alias as profile name\n"), currentAlias)
		}
	}

	// Validate we have all required credentials
	fmt.Println("\n" + strings.Repeat("-", 50))
	fmt.Println("VALIDATION")
	fmt.Println(strings.Repeat("-", 50))

	missingFields := []string{}
	if accounts.AccessKeyId == "" {
		missingFields = append(missingFields, "aws_access_key_id")
	}
	if accounts.SecretAccessKey == "" {
		missingFields = append(missingFields, "aws_secret_access_key")
	}
	if accounts.SessionToken == "" {
		missingFields = append(missingFields, "aws_session_token")
	}
	if currentAlias == "" {
		missingFields = append(missingFields, "account section header")
	}

	if len(missingFields) > 0 {
		fmt.Printf(updater.Fatal("✗ Missing required fields: %s\n"), strings.Join(missingFields, ", "))
		fmt.Print(updater.Warn("⚠ Credentials not written to file\n"))
		return
	}

	fmt.Print(updater.Green("✓ All required fields present\n"))

	// Write credentials to AWS credentials file if we have all required data
	if accounts.AccessKeyId != "" && accounts.SecretAccessKey != "" && accounts.SessionToken != "" && currentAlias != "" {
		// Expand the AWS secrets file path
		awsCredsFile := config.AWSSecretsFile

		if strings.HasPrefix(awsCredsFile, "~/") {
			// Handle ~/path format - expand to user home directory
			userHomeDir, err := updater.GetLocalUserHomeDir()
			if err != nil {
				fmt.Printf(updater.Fatal("\U00002717 Could not determine home directory: %s\n"), err.Error())
				return
			}
			awsCredsFile = filepath.Join(userHomeDir, awsCredsFile[2:])
		} else if strings.HasPrefix(awsCredsFile, "./") || strings.HasPrefix(awsCredsFile, "../") || !strings.Contains(awsCredsFile, "/") {
			// Handle local relative paths - use current directory (for testing)
			// Examples: ./credentials, ../test/creds, credentials
			// Leave as-is - will be relative to current working directory
		} else if !filepath.IsAbs(awsCredsFile) {
			// Handle other relative paths - default to ~/.aws/ directory
			userHomeDir, err := updater.GetLocalUserHomeDir()
			if err != nil {
				fmt.Printf(updater.Fatal("\U00002717 Could not determine home directory: %s\n"), err.Error())
				return
			}
			awsCredsFile = filepath.Join(userHomeDir, ".aws", awsCredsFile)
		}
		// Absolute paths are used as-is

		fmt.Printf("\nWriting credentials to: %s\n", awsCredsFile)

		if awsCredsFile == "" {
			fmt.Print(updater.Fatal("\U00002717 AWS credentials file path is empty\n"))
			return
		}

		if err := updater.WriteCreds(awsCredsFile, accountName, accounts); err != nil {
			fmt.Printf(updater.Fatal("\U00002717 Error writing credentials: %s\n"), err.Error())
		} else {
			if accountName != currentAlias {
				fmt.Printf(updater.Green("\U00002713 Successfully wrote credentials for profile '%s' (from alias '%s')\n"), accountName, currentAlias)
			} else {
				fmt.Printf(updater.Green("\U00002713 Successfully wrote credentials for profile '%s'\n"), accountName)
			}
		}
	} else {
		fmt.Print(updater.Warn("\U000026A0 Missing required data - credentials not written\n"))
	}

}
