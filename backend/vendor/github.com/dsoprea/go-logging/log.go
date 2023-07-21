package log

import (
	"bytes"
	e "errors"
	"fmt"
	"strings"
	"sync"

	"text/template"

	"github.com/go-errors/errors"
	"golang.org/x/net/context"
)

// TODO(dustin): Finish symbol documentation

// Config severity integers.
const (
	LevelDebug   = iota
	LevelInfo    = iota
	LevelWarning = iota
	LevelError   = iota
)

// Config severity names.
const (
	LevelNameDebug   = "debug"
	LevelNameInfo    = "info"
	LevelNameWarning = "warning"
	LevelNameError   = "error"
)

// Seveirty name->integer map.
var (
	LevelNameMap = map[string]int{
		LevelNameDebug:   LevelDebug,
		LevelNameInfo:    LevelInfo,
		LevelNameWarning: LevelWarning,
		LevelNameError:   LevelError,
	}

	LevelNameMapR = map[int]string{
		LevelDebug:   LevelNameDebug,
		LevelInfo:    LevelNameInfo,
		LevelWarning: LevelNameWarning,
		LevelError:   LevelNameError,
	}
)

// Errors
var (
	ErrAdapterAlreadyRegistered = e.New("adapter already registered")
	ErrFormatEmpty              = e.New("format is empty")
	ErrExcludeLevelNameInvalid  = e.New("exclude bypass-level is invalid")
	ErrNoAdapterConfigured      = e.New("no default adapter configured")
	ErrAdapterIsNil             = e.New("adapter is nil")
	ErrConfigurationNotLoaded   = e.New("can not configure because configuration is not loaded")
)

// Other
var (
	includeFilters    = make(map[string]bool)
	useIncludeFilters = false
	excludeFilters    = make(map[string]bool)
	useExcludeFilters = false

	adapters = make(map[string]LogAdapter)

	// TODO(dustin): !! Finish implementing this.
	excludeBypassLevel = -1
)

// Add global include filter.
func AddIncludeFilter(noun string) {
	includeFilters[noun] = true
	useIncludeFilters = true
}

// Remove global include filter.
func RemoveIncludeFilter(noun string) {
	delete(includeFilters, noun)
	if len(includeFilters) == 0 {
		useIncludeFilters = false
	}
}

// Add global exclude filter.
func AddExcludeFilter(noun string) {
	excludeFilters[noun] = true
	useExcludeFilters = true
}

// Remove global exclude filter.
func RemoveExcludeFilter(noun string) {
	delete(excludeFilters, noun)
	if len(excludeFilters) == 0 {
		useExcludeFilters = false
	}
}

func AddAdapter(name string, la LogAdapter) {
	if _, found := adapters[name]; found == true {
		Panic(ErrAdapterAlreadyRegistered)
	}

	if la == nil {
		Panic(ErrAdapterIsNil)
	}

	adapters[name] = la

	if GetDefaultAdapterName() == "" {
		SetDefaultAdapterName(name)
	}
}

func ClearAdapters() {
	adapters = make(map[string]LogAdapter)
	SetDefaultAdapterName("")
}

type LogAdapter interface {
	Debugf(lc *LogContext, message *string) error
	Infof(lc *LogContext, message *string) error
	Warningf(lc *LogContext, message *string) error
	Errorf(lc *LogContext, message *string) error
}

// TODO(dustin): !! Also populate whether we've bypassed an exception so that
//                  we can add a template macro to prefix an exclamation of
//                  some sort.
type MessageContext struct {
	Level         *string
	Noun          *string
	Message       *string
	ExcludeBypass bool
}

type LogContext struct {
	Logger *Logger
	Ctx    context.Context
}

type Logger struct {
	isConfigured bool
	an           string
	la           LogAdapter
	t            *template.Template
	systemLevel  int
	noun         string
}

func NewLoggerWithAdapterName(noun string, adapterName string) (l *Logger) {
	l = &Logger{
		noun: noun,
		an:   adapterName,
	}

	return l
}

func NewLogger(noun string) (l *Logger) {
	l = NewLoggerWithAdapterName(noun, "")

	return l
}

func (l *Logger) Noun() string {
	return l.noun
}

func (l *Logger) Adapter() LogAdapter {
	return l.la
}

var (
	configureMutex sync.Mutex
)

func (l *Logger) doConfigure(force bool) {
	configureMutex.Lock()
	defer configureMutex.Unlock()

	if l.isConfigured == true && force == false {
		return
	}

	if IsConfigurationLoaded() == false {
		Panic(ErrConfigurationNotLoaded)
	}

	if l.an == "" {
		l.an = GetDefaultAdapterName()
	}

	// If this is empty, then no specific adapter was given or no system
	// default was configured (which implies that no adapters were registered).
	// All of our logging will be skipped.
	if l.an != "" {
		la, found := adapters[l.an]
		if found == false {
			Panic(fmt.Errorf("adapter is not valid: %s", l.an))
		}

		l.la = la
	}

	// Set the level.

	systemLevel, found := LevelNameMap[levelName]
	if found == false {
		Panic(fmt.Errorf("log-level not valid: [%s]", levelName))
	}

	l.systemLevel = systemLevel

	// Set the form.

	if format == "" {
		Panic(ErrFormatEmpty)
	}

	if t, err := template.New("logItem").Parse(format); err != nil {
		Panic(err)
	} else {
		l.t = t
	}

	l.isConfigured = true
}

func (l *Logger) flattenMessage(lc *MessageContext, format *string, args []interface{}) (string, error) {
	m := fmt.Sprintf(*format, args...)

	lc.Message = &m

	var b bytes.Buffer
	if err := l.t.Execute(&b, *lc); err != nil {
		return "", err
	}

	return b.String(), nil
}

func (l *Logger) allowMessage(noun string, level int) bool {
	if _, found := includeFilters[noun]; found == true {
		return true
	}

	// If we didn't hit an include filter and we *had* include filters, filter
	// it out.
	if useIncludeFilters == true {
		return false
	}

	if _, found := excludeFilters[noun]; found == true {
		return false
	}

	return true
}

func (l *Logger) makeLogContext(ctx context.Context) *LogContext {
	return &LogContext{
		Ctx:    ctx,
		Logger: l,
	}
}

type LogMethod func(lc *LogContext, message *string) error

func (l *Logger) log(ctx context.Context, level int, lm LogMethod, format string, args []interface{}) error {
	if l.systemLevel > level {
		return nil
	}

	// Preempt the normal filter checks if we can unconditionally allow at a
	// certain level and we've hit that level.
	//
	// Notice that this is only relevant if the system-log level is letting
	// *anything* show logs at the level we came in with.
	canExcludeBypass := level >= excludeBypassLevel && excludeBypassLevel != -1
	didExcludeBypass := false

	n := l.Noun()

	if l.allowMessage(n, level) == false {
		if canExcludeBypass == false {
			return nil
		} else {
			didExcludeBypass = true
		}
	}

	levelName, found := LevelNameMapR[level]
	if found == false {
		Panic(fmt.Errorf("level not valid: (%d)", level))
	}

	levelName = strings.ToUpper(levelName)

	lc := &MessageContext{
		Level:         &levelName,
		Noun:          &n,
		ExcludeBypass: didExcludeBypass,
	}

	if s, err := l.flattenMessage(lc, &format, args); err != nil {
		return err
	} else {
		lc := l.makeLogContext(ctx)
		if err := lm(lc, &s); err != nil {
			panic(err)
		}

		return e.New(s)
	}
}

func (l *Logger) Debugf(ctx context.Context, format string, args ...interface{}) {
	l.doConfigure(false)

	if l.la != nil {
		l.log(ctx, LevelDebug, l.la.Debugf, format, args)
	}
}

func (l *Logger) Infof(ctx context.Context, format string, args ...interface{}) {
	l.doConfigure(false)

	if l.la != nil {
		l.log(ctx, LevelInfo, l.la.Infof, format, args)
	}
}

func (l *Logger) Warningf(ctx context.Context, format string, args ...interface{}) {
	l.doConfigure(false)

	if l.la != nil {
		l.log(ctx, LevelWarning, l.la.Warningf, format, args)
	}
}

func (l *Logger) mergeStack(err interface{}, format string, args []interface{}) (string, []interface{}) {
	if format != "" {
		format += "\n%s"
	} else {
		format = "%s"
	}

	var stackified *errors.Error
	stackified, ok := err.(*errors.Error)
	if ok == false {
		stackified = errors.Wrap(err, 2)
	}

	args = append(args, stackified.ErrorStack())

	return format, args
}

func (l *Logger) Errorf(ctx context.Context, errRaw interface{}, format string, args ...interface{}) {
	l.doConfigure(false)

	var err interface{}

	if errRaw != nil {
		_, ok := errRaw.(*errors.Error)
		if ok == true {
			err = errRaw
		} else {
			err = errors.Wrap(errRaw, 1)
		}
	}

	if l.la != nil {
		if errRaw != nil {
			format, args = l.mergeStack(err, format, args)
		}

		l.log(ctx, LevelError, l.la.Errorf, format, args)
	}
}

func (l *Logger) ErrorIff(ctx context.Context, errRaw interface{}, format string, args ...interface{}) {
	if errRaw == nil {
		return
	}

	var err interface{}

	_, ok := errRaw.(*errors.Error)
	if ok == true {
		err = errRaw
	} else {
		err = errors.Wrap(errRaw, 1)
	}

	l.Errorf(ctx, err, format, args...)
}

func (l *Logger) Panicf(ctx context.Context, errRaw interface{}, format string, args ...interface{}) {
	l.doConfigure(false)

	var err interface{}

	_, ok := errRaw.(*errors.Error)
	if ok == true {
		err = errRaw
	} else {
		err = errors.Wrap(errRaw, 1)
	}

	if l.la != nil {
		format, args = l.mergeStack(err, format, args)
		err = l.log(ctx, LevelError, l.la.Errorf, format, args)
	}

	Panic(err.(error))
}

func (l *Logger) PanicIff(ctx context.Context, errRaw interface{}, format string, args ...interface{}) {
	if errRaw == nil {
		return
	}

	var err interface{}

	_, ok := errRaw.(*errors.Error)
	if ok == true {
		err = errRaw
	} else {
		err = errors.Wrap(errRaw, 1)
	}

	l.Panicf(ctx, err.(error), format, args...)
}

func Wrap(err interface{}) *errors.Error {
	es, ok := err.(*errors.Error)
	if ok == true {
		return es
	} else {
		return errors.Wrap(err, 1)
	}
}

func Errorf(message string, args ...interface{}) *errors.Error {
	err := fmt.Errorf(message, args...)
	return errors.Wrap(err, 1)
}

func Panic(err interface{}) {
	_, ok := err.(*errors.Error)
	if ok == true {
		panic(err)
	} else {
		panic(errors.Wrap(err, 1))
	}
}

func Panicf(message string, args ...interface{}) {
	err := Errorf(message, args...)
	Panic(err)
}

func PanicIf(err interface{}) {
	if err == nil {
		return
	}

	_, ok := err.(*errors.Error)
	if ok == true {
		panic(err)
	} else {
		panic(errors.Wrap(err, 1))
	}
}

// Is checks if the left ("actual") error equals the right ("against") error.
// The right must be an unwrapped error (the kind that you'd initialize as a
// global variable). The left can be a wrapped or unwrapped error.
func Is(actual, against error) bool {
	// If it's an unwrapped error.
	if _, ok := actual.(*errors.Error); ok == false {
		return actual == against
	}

	return errors.Is(actual, against)
}

// Print is a utility function to prevent the caller from having to import the
// third-party library.
func PrintError(err error) {
	wrapped := Wrap(err)
	fmt.Printf("Stack:\n\n%s\n", wrapped.ErrorStack())
}

// PrintErrorf is a utility function to prevent the caller from having to
// import the third-party library.
func PrintErrorf(err error, format string, args ...interface{}) {
	wrapped := Wrap(err)

	fmt.Printf(format, args...)
	fmt.Printf("\n")
	fmt.Printf("Stack:\n\n%s\n", wrapped.ErrorStack())
}

func init() {
	if format == "" {
		format = defaultFormat
	}

	if levelName == "" {
		levelName = defaultLevelName
	}

	if includeNouns != "" {
		for _, noun := range strings.Split(includeNouns, ",") {
			AddIncludeFilter(noun)
		}
	}

	if excludeNouns != "" {
		for _, noun := range strings.Split(excludeNouns, ",") {
			AddExcludeFilter(noun)
		}
	}

	if excludeBypassLevelName != "" {
		var found bool
		if excludeBypassLevel, found = LevelNameMap[excludeBypassLevelName]; found == false {
			panic(ErrExcludeLevelNameInvalid)
		}
	}
}
