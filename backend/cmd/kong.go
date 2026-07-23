package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/gtsteffaniak/filebrowser/backend/internal/version"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/term"
)

// Globals holds flags shared across all commands (order-independent).
type Globals struct {
	Config  string `short:"c" name:"config" default:"config.yaml" env:"FILEBROWSER_CONFIG" help:"Path to config file"`
	NoInput bool   `name:"no-input" help:"Disable interactive prompts (fail if input required)"`
}

type runCmd struct{}

type versionCmd struct{}

type setupCmd struct{}

type SetCmd struct {
	User  string     `short:"u" help:"Deprecated: comma-separated username,password. Use 'user set' instead."`
	Admin bool       `short:"a" help:"Create user as admin (used with -u)"`
	Rule  SetRuleCmd `cmd:"" name:"rule" help:"Add or update an access rule"`
}

type SetRuleCmd struct {
	Source string `short:"s" name:"source" aliases:"f,sourceName" required:"" help:"Source name from config"`
	Path   string `short:"p" name:"path" aliases:"sourcePath" required:"" help:"Index path within the source"`
	Role   string `short:"r" name:"role" enum:"user,group,all" required:"" help:"Rule target type"`
	Value  string `short:"v" name:"value" help:"Username or group name (required for user/group)"`
	Allow  bool   `name:"allow" help:"Allow access (default: deny)"`
}

type UserCmd struct {
	Set     UserSetCmd     `cmd:"" name:"set" help:"Create or update a password-authenticated user"`
	Promote UserPromoteCmd `cmd:"" name:"promote" help:"Grant admin permissions without changing password"`
}

type UserPromoteCmd struct {
	Username string `arg:"" help:"Username to promote"`
}

type UserSetCmd struct {
	Username string       `arg:"" help:"Username"`
	Password passwordFlag `name:"password" help:"Password; omit value to prompt (TTY) or read stdin (pipe)"`
	Admin    bool         `short:"a" name:"admin" help:"Grant admin permissions"`
}

// passwordFlag accepts --password VALUE (inline) or --password alone (prompt/pipe).
type passwordFlag struct {
	Provided bool
	Inline   string
}

func (p *passwordFlag) Decode(ctx *kong.DecodeContext) error {
	p.Provided = true
	if ctx.Scan.Peek().IsValue() {
		return ctx.Scan.PopValueInto("password", &p.Inline)
	}
	return nil
}

func (p passwordFlag) resolve(noInput bool) (string, error) {
	if !p.Provided {
		return "", fmt.Errorf("--password is required (inline value, interactive prompt, or piped stdin)")
	}
	if p.Inline != "" {
		return p.Inline, nil
	}
	if noInput {
		return "", fmt.Errorf("--password requires an inline value or piped stdin when --no-input is set")
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return promptPassword()
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	password := strings.TrimSpace(string(data))
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	return password, nil
}

func promptPassword() (string, error) {
	if _, err := fmt.Fprint(os.Stderr, "Password: "); err != nil {
		return "", err
	}
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	password := string(bytePassword)
	if password == "" {
		return "", fmt.Errorf("password must not be empty")
	}
	return password, nil
}

var rootCLI struct {
	Globals `embed:""`

	Run     runCmd     `cmd:"" default:"1" hidden:"" help:"Start the FileBrowser server"`
	Version versionCmd `cmd:"" name:"version" help:"Print version information"`
	Setup   setupCmd   `cmd:"" name:"setup" help:"Interactive configuration setup"`
	Set     SetCmd     `cmd:"" name:"set" help:"Set configuration values (deprecated: use 'user set' for users)"`
	User    UserCmd    `cmd:"" name:"user" help:"User management"`
}

func (versionCmd) Run() error {
	fmt.Printf(`FileBrowser Quantum - A modern web-based file manager
	Version 	 : %v
	Commit 		 : %v
	Release Info 	 : https://github.com/gtsteffaniak/filebrowser/releases/tag/%v
`, version.Version, version.CommitSHA, version.Version)
	return nil
}

func (setupCmd) Run(globals *Globals) error {
	createConfig(globals.Config)
	return nil
}

func (s *SetCmd) Run(ctx *kong.Context) error {
	if strings.HasPrefix(ctx.Command(), "set rule") {
		return nil
	}
	if s.User == "" {
		return fmt.Errorf("missing subcommand for 'set'. Use 'set rule' or 'user set'")
	}
	fmt.Fprintln(os.Stderr, "warning: 'set -u' is deprecated; use 'user set' instead")
	userInfo := strings.SplitN(s.User, ",", 2)
	if len(userInfo) < 2 || userInfo[0] == "" || userInfo[1] == "" {
		return fmt.Errorf("not enough info to create user: \"set -u username,password\", only provided %v", strings.Split(s.User, ","))
	}
	return setUser(userInfo[0], userInfo[1], s.Admin)
}

func (r *SetRuleCmd) Run() error {
	sourceInfo, ok := settings.Config.Server.NameToSource[r.Source]
	if !ok {
		return fmt.Errorf("invalid source name: %s", r.Source)
	}
	return setRule(sourceInfo.Path, r.Path, r.Role, r.Value, r.Allow)
}

func (u *UserSetCmd) Run(globals *Globals) error {
	password, err := u.Password.resolve(globals.NoInput)
	if err != nil {
		return err
	}
	return setUser(u.Username, password, u.Admin)
}

func (p *UserPromoteCmd) Run() error {
	return promoteUser(p.Username)
}

func resolveConfigPath(config *string) {
	envConfig := os.Getenv("FILEBROWSER_CONFIG")
	if *config == "" {
		if envConfig != "" {
			*config = envConfig
		} else {
			*config = "config.yaml"
		}
	}
	if envConfig != "" && *config == envConfig {
		if _, err := os.Stat(*config); err != nil {
			logger.Fatalf("config file %v does not exist, please create it or set the FILEBROWSER_CONFIG environment variable to a valid config file path", *config)
		}
	}
}

func runCLI() (keepGoing bool, dbExists bool) {
	generateYaml()

	parser, err := kong.New(&rootCLI,
		kong.Name("filebrowser"),
		kong.Description("FileBrowser Quantum - A modern web-based file manager"),
	)
	if err != nil {
		logger.Fatalf("failed to configure CLI: %v", err)
	}

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	resolveConfigPath(&rootCLI.Config)
	configPath = rootCLI.Config

	cmd := ctx.Command()
	switch {
	case cmd == "" || cmd == "run":
		dbExists = initializeDatabase(configPath)
		return true, dbExists
	case cmd == "version" || cmd == "setup":
		parser.FatalIfErrorf(ctx.Run(&rootCLI))
		return false, false
	case cmd == "set rule" || cmd == "set" || strings.HasPrefix(cmd, "user set") || strings.HasPrefix(cmd, "user promote"):
		dbExists = initializeDatabase(configPath)
		parser.FatalIfErrorf(ctx.Run(&rootCLI))
		return false, dbExists
	default:
		parser.Fatalf("unexpected command: %q", cmd)
		return false, false
	}
}
