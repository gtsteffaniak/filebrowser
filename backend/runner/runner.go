package runner

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

// Runner is a commands runner.
type Runner struct {
	Enabled bool
	*settings.Settings
}

// RunHook runs the hooks for the before and after event.
func (r *Runner) RunHook(fn func() error, evt, path, dst string, user *users.User) error {
	idx := files.GetIndex("default")
	path, _, _ = idx.GetRealPath(user.Scopes[idx.Name], path)
	dst, _, _ = idx.GetRealPath(user.Scopes[idx.Name], dst)
	err := fn()
	if err != nil {
		return err
	}
	return nil
}

func (r *Runner) exec(raw, evt, path, dst string, user *users.User) error {
	blocking := true

	if strings.HasSuffix(raw, "&") {
		blocking = false
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "&"))
	}

	command, err := ParseCommand(r.Settings, raw)
	if err != nil {
		return err
	}

	envMapping := func(key string) string {
		switch key {
		case "FILE":
			return path
		case "SCOPE":
			return user.Scopes["default"]
		case "TRIGGER":
			return evt
		case "USERNAME":
			return user.Username
		case "DESTINATION":
			return dst
		default:
			return os.Getenv(key)
		}
	}
	for i, arg := range command {
		if i == 0 {
			continue
		}

		command[i] = os.Expand(arg, envMapping)
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("FILE=%s", path))
	cmd.Env = append(cmd.Env, fmt.Sprintf("SCOPE=%s", user.Scopes["default"]))
	cmd.Env = append(cmd.Env, fmt.Sprintf("TRIGGER=%s", evt))
	cmd.Env = append(cmd.Env, fmt.Sprintf("USERNAME=%s", user.Username))
	cmd.Env = append(cmd.Env, fmt.Sprintf("DESTINATION=%s", dst))

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if !blocking {
		log.Printf("Nonblocking Command: \"%s\"", strings.Join(command, " "))
		defer func() {
			go func() {
				err := cmd.Wait()
				if err != nil {
					log.Printf("Nonblocking Command \"%s\" failed: %s", strings.Join(command, " "), err)
				}
			}()
		}()
		return cmd.Start()
	}

	log.Printf("Blocking Command: \"%s\"", strings.Join(command, " "))
	return cmd.Run()
}
