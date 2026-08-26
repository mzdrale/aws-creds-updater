package updater

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Accounts struct {
	Id              string
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
}

// WriteCreds writes AWS credentials to the specified file, updating existing profile or adding new one
func WriteCreds(file string, profileName string, account Accounts) error {
	// Create the new profile section (without trailing newline - we'll add it conditionally)
	newProfileLines := []string{
		fmt.Sprintf("[%s]", profileName),
		fmt.Sprintf("aws_access_key_id = %s", account.AccessKeyId),
		fmt.Sprintf("aws_secret_access_key = %s", account.SecretAccessKey),
		fmt.Sprintf("aws_session_token = %s", account.SessionToken),
	}

	var outputLines []string
	inTargetProfile := false
	lastLineWasBlank := false

	// Read existing file if it exists
	if _, err := os.Stat(file); err == nil {
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed to open credentials file for reading: %w", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			isBlank := strings.TrimSpace(line) == ""

			// Check if this is a profile header
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				// Extract profile name
				profileNameFromLine := strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")

				// If we were in the target profile, we've now moved to a new profile
				if inTargetProfile {
					inTargetProfile = false
				}

				// Check if this is the profile we want to update
				if profileNameFromLine == profileName {
					inTargetProfile = true
					// Don't add this line - we'll add the new profile section instead
					// Remove any trailing blank lines before this profile
					for len(outputLines) > 0 && strings.TrimSpace(outputLines[len(outputLines)-1]) == "" {
						outputLines = outputLines[:len(outputLines)-1]
					}
					lastLineWasBlank = false
					continue
				} else {
					// Remove trailing blank lines before adding new profile header
					for len(outputLines) > 0 && strings.TrimSpace(outputLines[len(outputLines)-1]) == "" {
						outputLines = outputLines[:len(outputLines)-1]
					}
					// Add this profile header
					outputLines = append(outputLines, line)
					lastLineWasBlank = false
				}
			} else if inTargetProfile {
				// Skip lines from the profile we're replacing
				continue
			} else if isBlank {
				// Only add blank line if previous line wasn't blank (to avoid multiple blank lines)
				if !lastLineWasBlank && len(outputLines) > 0 {
					outputLines = append(outputLines, line)
					lastLineWasBlank = true
				}
			} else {
				// Add line to output (from other profiles or before any profile)
				outputLines = append(outputLines, line)
				lastLineWasBlank = false
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read credentials file: %w", err)
		}
	}

	// Remove trailing blank lines
	for len(outputLines) > 0 && strings.TrimSpace(outputLines[len(outputLines)-1]) == "" {
		outputLines = outputLines[:len(outputLines)-1]
	}

	// Write the file
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open credentials file for writing: %w", err)
	}
	defer f.Close()

	// Write existing profiles (excluding the one we're updating)
	for i, line := range outputLines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("failed to write line: %w", err)
		}

		// Add a blank line after each profile section (when we see the next profile header)
		if i < len(outputLines)-1 {
			nextLine := outputLines[i+1]
			if strings.HasPrefix(nextLine, "[") && strings.HasSuffix(nextLine, "]") {
				// Next line is a profile header, add blank line after current profile
				// But only if current line is not already blank
				if strings.TrimSpace(line) != "" {
					if _, err := fmt.Fprintln(f); err != nil {
						return fmt.Errorf("failed to write blank line: %w", err)
					}
				}
			}
		}
	}

	// Add blank line before new profile if there are existing profiles
	if len(outputLines) > 0 {
		if _, err := fmt.Fprintln(f); err != nil {
			return fmt.Errorf("failed to write blank line: %w", err)
		}
	}

	// Write the new/updated profile
	for _, line := range newProfileLines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("failed to write new credentials: %w", err)
		}
	}

	return nil
}
