[![Build Status](https://travis-ci.org/dsoprea/go-logging.svg?branch=master)](https://travis-ci.org/dsoprea/go-logging)
[![Coverage Status](https://coveralls.io/repos/github/dsoprea/go-logging/badge.svg?branch=master)](https://coveralls.io/github/dsoprea/go-logging?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/dsoprea/go-logging/v2)](https://goreportcard.com/report/github.com/dsoprea/go-logging/v2)
[![GoDoc](https://godoc.org/github.com/dsoprea/go-logging/v2?status.svg)](https://godoc.org/github.com/dsoprea/go-logging/v2)

## Introduction

This project bridges several gaps that are present in the standard logging support in Go:

- Equips errors with stacktraces and provides a facility for printing them
- Inherently supports the ability for each Go file to print its messages with a prefix representing that file/package
- Adds some functions to specifically log messages of different levels (e.g. debug, error)
- Adds a `PanicIf()` function that can be used to conditionally manage errors depending on whether an error variable is `nil` or actually has an error
- Adds support for pluggable logging adapters (so the output can be sent somewhere other than the console)
- Adds configuration (such as the logging level or adapter) that can be driven from the environment
- Supports filtering to show/hide the logging of certain places of the application
- The loggers can be definded at the package level, so you can determine which Go file any log message came from.

When used with the Panic-Defer-Recover pattern in Go, even panics rising from the Go runtime will be caught and wrapped with a stacktrace. This compartmentalizes which function they could have originated from, which is, otherwise, potentially non-trivial to figure out.

## AppEngine

Go under AppEngine is very stripped down, such as there being no logging type (e.g. `Logger` in native Go) and there is no support for prefixing. As each logging call from this project takes a `Context`, this works cooperatively to bridge the additional gaps in AppEngine's logging support.

With standard console logging outside of this context, that parameter will take a`nil`.


## Getting Started

The simplest, possible example:

```go
package thispackage

import (
    "context"
    "errors"

    "github.com/dsoprea/go-logging/v2"
)

var (
    thisfileLog = log.NewLogger("thispackage.thisfile")
)

func a_cry_for_help(ctx context.Context) {
    err := errors.New("a big error")
    thisfileLog.Errorf(ctx, err, "How big is my problem: %s", "pretty big")
}

func init() {
    cla := log.NewConsoleLogAdapter()
    log.AddAdapter("console", cla)
}
```

Notice two things:

1. We register the "console" adapter at the bottom. The first adapter registered will be used by default.
2. We pass-in a prefix (what we refer to as a "noun") to `log.NewLogger()`. This is a simple, descriptive name that represents the subject of the file. By convention, we construct this by dot-separating the current package and the name of the file. We recommend that you define a different log for every file at the package level, but it is your choice whether you want to do this or share the same logger over the entire package, define one in each struct, etc..


### Example Output

Example output from a real application (not from the above):

```
2016/09/09 12:57:44 DEBUG: user: User revisiting: [test@example.com]
2016/09/09 12:57:44 DEBUG: context: Session already inited: [DCRBDGRY6RMWANCSJXVLD7GULDH4NZEB6SBAQ3KSFIGA2LP45IIQ]
2016/09/09 12:57:44 DEBUG: session_data: Session save not necessary: [DCRBDGRY6RMWANCSJXVLD7GULDH4NZEB6SBAQ3KSFIGA2LP45IIQ]
2016/09/09 12:57:44 DEBUG: context: Got session: [DCRBDGRY6RMWANCSJXVLD7GULDH4NZEB6SBAQ3KSFIGA2LP45IIQ]
2016/09/09 12:57:44 DEBUG: session_data: Found user in session.
2016/09/09 12:57:44 DEBUG: cache: Cache miss: [geo.geocode.reverse:dhxp15x]
```


## Adapters

This project provides one built-in logging adapter, "console", which prints to the screen. To register it:

```go
cla := log.NewConsoleLogAdapter()
log.AddAdapter("console", cla)
```

### Custom Adapters

If you would like to implement your own logger, just create a struct type that satisfies the LogAdapter interface.

```go
type LogAdapter interface {
    Debugf(lc *LogContext, message *string) error
    Infof(lc *LogContext, message *string) error
    Warningf(lc *LogContext, message *string) error
    Errorf(lc *LogContext, message *string) error
}
```

The *LogContext* struct passed in provides additional information that you may need in order to do what you need to do:

```go
type LogContext struct {
    Logger *Logger
    Ctx context.Context
}
```

`Logger` represents your Logger instance.

Adapter example:

```go
type DummyLogAdapter struct {

}

func (dla *DummyLogAdapter) Debugf(lc *LogContext, message *string) error {

}

func (dla *DummyLogAdapter) Infof(lc *LogContext, message *string) error {

}

func (dla *DummyLogAdapter) Warningf(lc *LogContext, message *string) error {

}

func (dla *DummyLogAdapter) Errorf(lc *LogContext, message *string) error {

}
```

Then, register it:

```go
func init() {
    log.AddAdapter("dummy", new(DummyLogAdapter))
}
```

If this is a task-specific implementation, just register it from the `init()` of the file that defines it.

If this is the first adapter you've registered, it will be the default one used. Otherwise, you'll have to deliberately specify it when you are creating a logger: Instead of calling `log.NewLogger(noun string)`, call `log.NewLoggerWithAdapterName(noun string, adapterName string)`.

We discuss how to configure the adapter from configuration in the "Configuration" section below.


### Adapter Notes

- The `Logger` instance exports `Noun()` in the event you want to discriminate where your log entries go in your adapter. It also exports `Adapter()` for if you need to access the adapter instance from your application.
- If no adapter is registered (specifically, the default adapter-name remains empty), logging calls will be a no-op. This allows libraries to implement *go-logging* where the larger application doesn't.


## Filters

We support the ability to exclusively log for a specific set of nouns (we'll exclude any not specified):

```go
log.AddIncludeFilter("nountoshow1")
log.AddIncludeFilter("nountoshow2")
```

Depending on your needs, you might just want to exclude a couple and include the rest:

```go
log.AddExcludeFilter("nountohide1")
log.AddExcludeFilter("nountohide2")
```

We'll first hit the include-filters. If it's in there, we'll forward the log item to the adapter. If not, and there is at least one include filter in the list, we won't do anything. If the list of include filters is empty but the noun appears in the exclude list, we won't do anything.

It is a good convention to exclude the nouns of any library you are writing whose logging you do not want to generally be aware of unless you are debugging. You might call `AddExcludeFilter()` from the `init()` function at the bottom of those files unless there is some configuration variable, such as "(LibraryNameHere)DoShowLogging", that has been defined and set to TRUE.


## Configuration

The following configuration items are available:

- *Format*: The default format used to build the message that gets sent to the adapter. It is assumed that the adapter already prefixes the message with time and log-level (since the default AppEngine logger does). The default value is: `{{.Noun}}: [{{.Level}}] {{if eq .ExcludeBypass true}} [BYPASS]{{end}} {{.Message}}`. The available tokens are "Level", "Noun", "ExcludeBypass", and "Message".
- *DefaultAdapterName*: The default name of the adapter to use when NewLogger() is called (if this isn't defined then the name of the first registered adapter will be used).
- *LevelName*: The priority-level of messages permitted to be logged (all others will be discarded). By default, it is "info". Other levels are: "debug", "warning", "error", "critical"
- *IncludeNouns*: Comma-separated list of nouns to log for. All others will be ignored.
- *ExcludeNouns*: Comma-separated list on nouns to exclude from logging.
- *ExcludeBypassLevelName*: The log-level at which we will show logging for nouns that have been excluded. Allows you to hide excessive, unimportant logging for nouns but to still see their warnings, errors, etc...


### Configuration Providers

You provide the configuration by setting a configuration-provider. Configuration providers must satisfy the `ConfigurationProvider` interface. The following are provided with the project:

- `EnvironmentConfigurationProvider`: Read values from the environment.
- `StaticConfigurationProvider`: Set values directly on the struct.

**The configuration provider must be applied before doing any logging (otherwise it will have no effect).**

Environments such as AppEngine work best with `EnvironmentConfigurationProvider` as this is generally how configuration is exposed *by* AppEngine *to* the application. You can define this configuration directly in *that* configuration.

By default, no configuration-provider is applied, the level is defaulted to INFO and the format is defaulted to "{{.Noun}}:{{if eq .ExcludeBypass true}} [BYPASS]{{end}} {{.Message}}".

Again, if a configuration-provider does not provide a log-level or format, they will be defaulted (or left alone, if already set). If it does not provide an adapter-name, the adapter-name of the first registered adapter will be used.

Usage instructions of both follow.


### Environment-Based Configuration

```go
ecp := log.NewEnvironmentConfigurationProvider()
log.LoadConfiguration(ecp)
```

Each of the items listed at the top of the "Configuration" section can be specified in the environment using a prefix of "Log" (e.g. LogDefaultAdapterName).


### Static Configuration

```go
scp := log.NewStaticConfigurationProvider()
scp.SetLevelName(log.LevelNameWarning)

log.LoadConfiguration(scp)
```
