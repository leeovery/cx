package resolver

import (
	"errors"
	"strings"
)

var ErrZoxideNotInstalled = errors.New("zoxide is not installed")

var ErrNoMatch = errors.New("no match found")

type LookPathFunc func(file string) (string, error)

type ZoxideResolver struct {
	runner   CommandRunner
	lookPath LookPathFunc
}

func NewZoxideResolver(runner CommandRunner, lookPath LookPathFunc) *ZoxideResolver {
	return &ZoxideResolver{
		runner:   runner,
		lookPath: lookPath,
	}
}

// Query returns zoxide's best match for terms, ErrZoxideNotInstalled when zoxide
// is not on PATH, or ErrNoMatch when it exits non-zero.
func (z *ZoxideResolver) Query(terms string) (string, error) {
	if _, err := z.lookPath("zoxide"); err != nil {
		return "", ErrZoxideNotInstalled
	}

	parts := strings.Fields(terms)
	args := append([]string{"query"}, parts...)
	output, err := z.runner.Run("zoxide", args...)
	if err != nil {
		return "", ErrNoMatch
	}

	return strings.TrimSpace(output), nil
}
