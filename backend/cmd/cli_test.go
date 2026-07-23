package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCLIParser(t *testing.T) *kong.Kong {
	t.Helper()
	parser, err := kong.New(&rootCLI, kong.Name("filebrowser"), kong.Bind(&rootCLI.Globals))
	require.NoError(t, err)
	return parser
}

func TestParseDefaultServerCommand(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{})
	require.NoError(t, err)
	assert.Equal(t, "run", ctx.Command())
}

func TestParseVersionCommand(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{"version"})
	require.NoError(t, err)
	assert.Equal(t, "version", ctx.Command())
}

func TestParseGlobalConfigFlag(t *testing.T) {
	parser := newCLIParser(t)
	_, err := parser.Parse([]string{"-c", "/tmp/custom.yaml", "version"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/custom.yaml", rootCLI.Config)
}

func TestParseUserSetWithPasswordFlag(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{"user", "set", "alice", "--password", "secret", "--admin"})
	require.NoError(t, err)
	assert.Equal(t, "user set <username>", ctx.Command())
	assert.Equal(t, "alice", rootCLI.User.Set.Username)
	assert.True(t, rootCLI.User.Set.Password.Provided)
	assert.Equal(t, "secret", rootCLI.User.Set.Password.Inline)
	assert.True(t, rootCLI.User.Set.Admin)
}

func TestParseUserSetConfigFlagOrder(t *testing.T) {
	parser := newCLIParser(t)
	_, err := parser.Parse([]string{"-c", "other.yaml", "user", "set", "bob", "--password", "x"})
	require.NoError(t, err)
	assert.Equal(t, "other.yaml", rootCLI.Config)
}

func TestParseSetRuleFlags(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{
		"set", "rule",
		"-s", "access", "-p", "/", "-r", "user", "-v", "admin", "--allow",
	})
	require.NoError(t, err)
	assert.Equal(t, "set rule", ctx.Command())
	assert.Equal(t, "access", rootCLI.Set.Rule.Source)
	assert.Equal(t, "/", rootCLI.Set.Rule.Path)
	assert.Equal(t, "user", rootCLI.Set.Rule.Role)
	assert.Equal(t, "admin", rootCLI.Set.Rule.Value)
	assert.True(t, rootCLI.Set.Rule.Allow)
}

func TestParseLegacySetUser(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{"set", "-u", "alice,secret", "-a"})
	require.NoError(t, err)
	assert.Equal(t, "set", ctx.Command())
	assert.Equal(t, "alice,secret", rootCLI.Set.User)
	assert.True(t, rootCLI.Set.Admin)
}

func TestPasswordFlagResolveInline(t *testing.T) {
	p := passwordFlag{Provided: true, Inline: "inline-pass"}
	got, err := p.resolve(true)
	require.NoError(t, err)
	assert.Equal(t, "inline-pass", got)
}

func TestPasswordFlagResolveStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	_, err = w.WriteString("piped-secret\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	p := passwordFlag{Provided: true}
	got, err := p.resolve(false)
	require.NoError(t, err)
	assert.Equal(t, "piped-secret", got)
}

func TestPasswordFlagMissing(t *testing.T) {
	p := passwordFlag{}
	_, err := p.resolve(false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--password is required")
}

func TestPasswordFlagNoInputRequiresInline(t *testing.T) {
	p := passwordFlag{Provided: true}
	_, err := p.resolve(true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--no-input")
}

func TestSplitByMultiple(t *testing.T) {
	assert.Equal(t, []string{"info", "warning", "error"}, SplitByMultiple("info|warning|error"))
}

func TestSetCmdRunSkipsWhenRuleSelected(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{"set", "rule", "-s", "x", "-p", "/", "-r", "all"})
	require.NoError(t, err)
	setCmd := &rootCLI.Set
	setCmd.User = "should-not-run"
	err = setCmd.Run(ctx)
	require.NoError(t, err)
}

func TestSetCmdRunLegacyUser(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{"set", "-u", "bad"})
	require.NoError(t, err)
	setCmd := &rootCLI.Set
	err = setCmd.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enough info")
}

func TestPasswordReadAllFromReader(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("  trimmed  \n")
	data, err := io.ReadAll(&buf)
	require.NoError(t, err)
	assert.Equal(t, "trimmed", strings.TrimSpace(string(data)))
}

func TestResolveConfigPathEnvMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Setenv("FILEBROWSER_CONFIG", missing)
	rootCLI.Config = ""
	resolveConfigPath(&rootCLI.Config)
	assert.Equal(t, missing, rootCLI.Config)
	_, err := os.Stat(rootCLI.Config)
	assert.Error(t, err)
}

func TestSetupNoInputRejected(t *testing.T) {
	parser := newCLIParser(t)
	ctx, err := parser.Parse([]string{"setup", "--no-input"})
	require.NoError(t, err)
	err = ctx.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive input")
}
