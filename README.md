# aws-creds-updater

A command-line tool to update AWS credentials in your `~/.aws/credentials` file by parsing temporary credentials from stdin and mapping them to friendly profile names.

## Disclaimer

Use this software at your own risk.  No author or contributor is responsible for your use or any damage it may cause.

## Features

- 📝 **Parse AWS credentials** from stdin in standard AWS credential format
- 🔄 **Update existing profiles** or create new ones in your AWS credentials file
- 🏷️ **Map account aliases** to friendly profile names via configuration
- ✅ **Validation** of input credentials before writing
- 🛡️ **Safe updates** - replaces existing profiles instead of creating duplicates
- 📋 **Clean formatting** - maintains proper spacing between profile sections
- 🙈 **Masked display** when pasting interactively (first 5 and last 5 characters visible)

## Installation

### Build from source

```bash
# Clone the repository
git clone https://github.com/mzdrale/aws-creds-updater.git
cd aws-creds-updater

# Build the binary (using Make)
make build
# or using Task
task build
# or directly with go
go build -o aws-creds-updater cmd/aws-creds-updater/main.go

# Install (optional)
make install
# or
task install
# or
go install ./cmd/aws-creds-updater
```

### Using Make

This project includes a `Makefile` for build automation:

- `make build` or `make` - Build the binary
- `make run` - Compile and run the code
- `make test` - Run tests
- `make install` - Build and install to `~/go/bin/`
- `make update` - Update Go dependencies (`go get -u ./...` and `go mod tidy`)
- `make clean` - Remove build artifacts
- `make help` - Show all available targets

**Short aliases:**
- `make b` - Alias for `build`
- `make r` - Alias for `run`
- `make t` - Alias for `test`
- `make i` - Alias for `install`
- `make u` - Alias for `update`

**Cross-compilation:**
```bash
make build GOOS=linux GOARCH=amd64
```

### Using Task

Alternatively, this project supports [Task](https://taskfile.dev/) for build automation:

- `task build` - Build the binary
- `task run` - Compile and run the code
- `task test` - Run tests
- `task install` - Install to `~/go/bin/`
- `task update` - Update Go dependencies (`go get -u ./...` and `go mod tidy`)
- `task clean` - Remove build artifacts

**Short aliases:** `task b` (build), `task r` (run), `task t` (test), `task i` (install), `task u` (update)

## Configuration

Create a configuration file at `~/.config/aws-creds-updater/config.yml` or in the current directory as `config.yml`.

### Configuration Format

```yaml
profiles:
  - name: myorg-dev
    aliases:
      - 1111111111_Dev-Admin
  - name: myorg-staging
    aliases:
      - 2222222222_MYORG-FA-Admin
  - name: myorg-prod
    aliases:
      - 3333333333_MYORG-FA-Admin

aws_secrets_file: ~/.aws/credentials
```

### Configuration Options

- **`profiles`** - List of profile mappings
  - **`name`** - Friendly profile name to use in credentials file
  - **`aliases`** - List of account aliases that map to this profile name
- **`aws_secrets_file`** - Path to AWS credentials file
  - Supports `~/` for home directory expansion
  - Supports relative paths (e.g., `./credentials` for testing)
  - Defaults to `~/.aws/credentials` if relative path provided

### Example Configuration

See `config.yml.sample` for a complete example.

## Usage

### Basic Usage

1. Get your AWS temporary credentials (e.g., from AWS SSO, assume role, etc.)
2. Pipe them to `aws-creds-updater`:

```bash
cat <<EOF | ./aws-creds-updater
[1234567890_Dev-Admin]
aws_access_key_id=ASIABA4TBQPUTX5H87U7V
aws_secret_access_key=S6+5PnqTVXLM7draofpLBWFlo5nWer37apZgFK2YXXzy
aws_session_token=IQoJhGftUUJHGGFGH6gCXVzLXdle8QtMiJGMEQCIGYjSBCy

EOF
```

Or paste credentials interactively:

```bash
./aws-creds-updater
# Paste the credential block; the tool finishes on its own once all fields are present
```

When stdin is a terminal, each line is echoed with credential values masked (first 5 and last 5 characters visible). Long `aws_session_token` values paste correctly; you do not need extra blank Enters.

### Input Format

The tool expects AWS credentials in the following format:

```
[AccountID_Profile-Name]
aws_access_key_id=AKIA...
aws_secret_access_key=...
aws_session_token=...
```

### Command Line Options

```bash
./aws-creds-updater [OPTIONS]

Options:
  -V, --version    Print version information
```

## How It Works

1. **Reads configuration** from `~/.config/aws-creds-updater/config.yml` or `./config.yml`
2. **Parses credentials** from stdin
3. **Maps account alias** to friendly profile name (e.g., `1111111111_Dev-Admin` → `myorg-dev`)
4. **Updates credentials file**:
   - If profile exists: replaces it with new credentials
   - If profile doesn't exist: adds it as a new section
   - Preserves all other profiles unchanged
5. **Maintains clean formatting** with proper spacing between sections

## Examples

### Example 1: Update Existing Profile

```bash
$ echo "[1111111111_Dev-Admin]
aws_access_key_id=ASIABA4TFVPUTY7H37U7W
aws_secret_access_key=S6+5PnqRVXKM7drgdfpLNMFlo5nWer89apZgFK2YZZxt
aws_session_token=IQoJbGftBBJHGGFGH6gCXVzLXdlc3QtMiJGMEQCIGjYSBYc
" | ./aws-creds-updater

✓ Loaded config with 3 profiles from: /Users/user/.config/aws-creds-updater/config.yml
Ready to read AWS credentials from stdin (paste your credentials)...
(Credentials shown with first 5 and last 5 chars visible, rest masked)

==================================================
INPUT PROCESSING COMPLETE
==================================================
✓ Account section: 1111111111_Dev-Admin
✓ Access Key ID: ASIAB***********37U7W
✓ Secret Access Key: S6+5P**********************************YZZxt
✓ Session Token: IQoJb*************************************YSBYc
✓ Account mapping: 1111111111_Dev-Admin → myorg-dev

--------------------------------------------------
VALIDATION
--------------------------------------------------
✓ All required fields present

Writing credentials to: /Users/user/.aws/credentials
✓ Successfully wrote credentials for profile 'myorg-dev' (from alias '1111111111_Dev-Admin')
```

### Example 2: Profile Not in Config

If an account alias is not found in the configuration, the tool will:
- Show a warning
- Use the alias as the profile name
- Still write the credentials

```bash
$ echo "[256099319689_MYORG-FA-Admin]
aws_access_key_id=ASIABA4TFVPUTY7H37U7W
aws_secret_access_key=S6+5PnqRVXKM7drgdfpLNMFlo5nWer89apZgFK2YZZxt
aws_session_token=IQoJbGftBBJHGGFGH6gCXVzLXdlc3QtMiJGMEQCIGjYSBYc
" | ./aws-creds-updater

⚠ No config mapping found for '256099319689_MYORG-FA-Admin', using alias as profile name
✓ All required fields present
✓ Successfully wrote credentials for profile '256099319689_MYORG-FA-Admin'
```

### Example 3: Testing with Local File

For testing, you can use a local credentials file:

```yaml
# config.yml
aws_secrets_file: ./credentials
```

This allows you to test without modifying your actual `~/.aws/credentials` file.

## Output Format

The tool writes credentials in standard AWS credentials file format:

```ini
[profile-name]
aws_access_key_id = ASIABA4TFVPUTY7H37U7W
aws_secret_access_key = S6+5PnqRVXKM7drgdfpLNMFlo5nWer89apZgFK2YZZxt
aws_session_token = IQoJbGftBBJHGGFGH6gCXVzLXdlc3QtMiJGMEQCIGjYSBYc

[another-profile]
...
```

## Requirements

- Go 1.26 or later
- Make (optional, for build automation)
- Task (optional, alternative build automation tool)

## Dependencies

- [spf13/viper](https://github.com/spf13/viper) - Configuration management
- [spf13/pflag](https://github.com/spf13/pflag) - Command-line flag parsing
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) - Terminal raw-mode input (masked interactive paste)
- [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) - Non-blocking TTY reads

## Project Structure

```
aws-creds-updater/
├── cmd/
│   └── aws-creds-updater/
│       ├── main.go          # Main application entry point
│       └── read_test.go     # Tests for credential paste parsing
├── updater/                 # Internal package
│   ├── accounts.go          # Credential file operations (read/write)
│   ├── color.go             # Terminal color output utilities
│   └── common.go            # Common utilities (home directory, etc.)
├── config.yml.sample        # Example configuration file
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── Makefile                 # Make build configuration
├── taskfile.yaml            # Task build configuration (alternative)
├── VERSION                  # Version file
└── README.md                # This file
```

## License

[BSD 3-Clause License](LICENSE)

## Contributing

Contributions are welcome! Please follow these steps to contribute:

### Development Workflow

1. **Create your branch**

```bash
git checkout develop
git checkout -b <your_branch>
```

2. **Add your changes**

Make your code changes, following the existing code style and conventions.

3. **Run tests**

```bash
make test
# or
task test
```

4. **Commit your changes**

```bash
git add .
git commit -m "Description of your changes"
```

5. **Push your branch**

```bash
git push origin <your_branch>
```

6. **Create a Pull Request**

Create a Pull Request to merge `<your_branch>` into `develop` branch.

## Author

[Miroslav Zdrale](https://github.com/mzdrale)

