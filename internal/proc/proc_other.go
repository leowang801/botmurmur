//go:build !darwin

package proc

import "errors"

// NewLister returns a stub lister for platforms without a native
// implementation yet. It always returns an error so `botmurmur scan` fails
// loud and clear on unsupported platforms rather than silently reporting
// zero agents. Linux and Windows listers land in follow-up commits.
func NewLister() Lister {
	return &unsupportedLister{}
}

type unsupportedLister struct{}

func (u *unsupportedLister) List() ([]Process, []Warning, error) {
	return nil, nil, errors.New("botmurmur: this platform is not yet supported — macOS is the first target. Linux and Windows land in follow-up commits")
}

func (u *unsupportedLister) FetchEnv(pid int) (map[string]string, *Warning, error) {
	return nil, nil, errors.New("botmurmur: this platform is not yet supported")
}
