package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ConfigFileFlag struct {
	FlagSet   *flag.FlagSet
	LogPrintf func(format string, v ...interface{})
	value     string
}

var _ flag.Value = (*ConfigFileFlag)(nil)

func (c *ConfigFileFlag) String() string { return c.value }

func (c *ConfigFileFlag) Set(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()

	if c.LogPrintf != nil {
		c.LogPrintf("loading configuration file: %s", fn)
	}

	err = c.readConfig(f)
	if err != nil {
		return err
	}
	c.value = fn
	return nil
}

// ReadDefault reads the default configuration file from UserConfigDir. Missing default config file is silently ignored.
// Must be called before adding ConfigFileFlag to FlagSet because it sets the default value.
func (c *ConfigFileFlag) ReadDefault(program string) error {
	path, err := os.UserConfigDir()
	if err != nil {
		return nil
	}

	fn := filepath.Join(path, program, program+".conf")

	f, err := os.Open(fn)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // ignore missing default config file
		}
		return err
	}
	defer f.Close()

	if c.LogPrintf != nil {
		c.LogPrintf("loading configuration file: %s", fn)
	}

	err = c.readConfig(f)
	if err != nil {
		return err
	}

	c.value = fn
	return nil
}

func (c *ConfigFileFlag) readConfig(f io.Reader) error {
	scan := bufio.NewScanner(f)

	for scan.Scan() {
		line := scan.Text()
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue // comment
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid config line: %s", line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		err := c.FlagSet.Set(key, value)
		if err != nil {
			return err
		}
	}
	return nil
}
