package proc

// FakeLister is a test double that returns a fixed list of processes and env
// maps. It lets the scan pipeline be unit-tested without any platform code.
type FakeLister struct {
	Processes     []Process
	Envs          map[int]map[string]string // PID -> env
	ListWarnings  []Warning
	FetchWarnings map[int]*Warning
	FetchErrors   map[int]error
}

func (f *FakeLister) List() ([]Process, []Warning, error) {
	return f.Processes, f.ListWarnings, nil
}

func (f *FakeLister) FetchEnv(pid int) (map[string]string, *Warning, error) {
	if err, ok := f.FetchErrors[pid]; ok {
		return nil, nil, err
	}
	if w, ok := f.FetchWarnings[pid]; ok {
		return nil, w, nil
	}
	if env, ok := f.Envs[pid]; ok {
		return env, nil, nil
	}
	return map[string]string{}, nil, nil
}
