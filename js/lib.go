package js

// Lib is a named library that can be installed into a Runtime.
//
// A Lib is either a pure JavaScript library (see SourceLib and ProgramLib) or
// a host capability implemented in Go (e.g. jshttp, jssql). Its name is the
// unique key within an Engine and, by convention, matches the global binding
// the library installs.
//
// Implementations must be safe to install into multiple runtimes: a Lib holds
// shared, goroutine-safe dependencies (an http.Client, an orm.DB), never
// per-runtime state. Host libraries must issue IO through the context of the
// runtime they were installed into (Runtime.Context), so cancellation reaches
// blocking Go calls.
type Lib interface {
	// Name returns the library's unique identifier within an Engine.
	Name() string
	// Install binds the library into the runtime.
	Install(rt *Runtime) error
}

// programLib is a Lib backed by a compiled JavaScript program.
type programLib struct {
	name    string
	program *Program
}

// ProgramLib builds a Lib from a pre-compiled JavaScript program.
func ProgramLib(name string, program *Program) Lib {
	return &programLib{name: name, program: program}
}

// SourceLib builds a Lib from JavaScript source, compiling it eagerly so
// syntax errors surface at construction time rather than at install time.
func SourceLib(name, source string) (Lib, error) {
	program, err := Compile(name, source, true)
	if err != nil {
		return nil, err
	}

	return ProgramLib(name, program), nil
}

func (l *programLib) Name() string {
	return l.name
}

func (l *programLib) Install(rt *Runtime) error {
	_, err := rt.vm.RunProgram(l.program)

	return err
}
