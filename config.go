package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Views map[string]View `yaml:"views"`
}

type View struct {
	Title string     `yaml:"title"`
	Form  []FormStep `yaml:"steps"`
	Run   string     `yaml:"run"`
	Union []string   `yaml:"union"`
	Menu  []string   `yaml:"menu"`
}

type Source struct {
	List    string `yaml:"list"`
	Display string `yaml:"display"`
	Preview string `yaml:"preview"`
	Input   string `yaml:"input"`
	Label   string `yaml:"label"`
}

type FormStep struct {
	Name    string   `yaml:"name"`
	Sources []Source `yaml:"sources"`
}

func (s *FormStep) listSources() []Source {
	var result []Source
	for _, src := range s.Sources {
		if src.List != "" {
			result = append(result, src)
		}
	}
	return result
}

func (s *FormStep) inputSources() []Source {
	var result []Source
	for _, src := range s.Sources {
		if src.Input != "" {
			result = append(result, src)
		}
	}
	return result
}

func (s *FormStep) isInputOnly() bool {
	return len(s.listSources()) == 0 && len(s.inputSources()) > 0
}

func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (v *View) isFormView() bool {
	return v.Run != ""
}

func (v *View) isUnionView() bool {
	return len(v.Union) > 0
}

func (v *View) isMenuView() bool {
	return len(v.Menu) > 0
}

func (c *Config) validateUnionRef(viewName, ref string) error {
	target, ok := c.Views[ref]
	if !ok {
		return fmt.Errorf("view %q: union references unknown view %q", viewName, ref)
	}
	if target.isUnionView() {
		for _, innerRef := range target.Union {
			if err := c.validateUnionRef(viewName, innerRef); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func (c *Config) validate() error {
	for name, v := range c.Views {
		if err := v.validate(name); err != nil {
			return err
		}
	}
	for name, v := range c.Views {
		for _, ref := range v.Union {
			if err := c.validateUnionRef(name, ref); err != nil {
				return err
			}
		}
		for _, ref := range v.Menu {
			if _, ok := c.Views[ref]; !ok {
				return fmt.Errorf("view %q: menu references unknown view %q", name, ref)
			}
		}
	}
	return nil
}

func (v *View) validate(name string) error {
	hasRun := v.Run != ""
	hasUnion := len(v.Union) > 0
	hasMenu := len(v.Menu) > 0

	count := 0
	if hasRun {
		count++
	}
	if hasUnion {
		count++
	}
	if hasMenu {
		count++
	}

	if count == 0 {
		return fmt.Errorf("view %q: must have one of run, union, or menu", name)
	}
	if count > 1 {
		return fmt.Errorf("view %q: must have only one of run, union, or menu", name)
	}

	for i, step := range v.Form {
		if err := step.validate(name, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *FormStep) validate(viewName string, index int) error {
	if s.Name == "" {
		return fmt.Errorf("view %q: form step %d: name is required", viewName, index)
	}
	if len(s.Sources) == 0 {
		return fmt.Errorf("view %q: form step %q: sources is required", viewName, s.Name)
	}
	for i, src := range s.Sources {
		if err := src.validate(viewName, s.Name, i); err != nil {
			return err
		}
	}
	return nil
}

func (src *Source) validate(viewName, stepName string, index int) error {
	hasList := src.List != ""
	hasInput := src.Input != ""

	if !hasList && !hasInput {
		return fmt.Errorf("view %q: form step %q: source %d: must have list or input", viewName, stepName, index)
	}
	if hasList && hasInput {
		return fmt.Errorf("view %q: form step %q: source %d: cannot have both list and input", viewName, stepName, index)
	}

	if hasList {
		if src.Label != "" {
			return fmt.Errorf("view %q: form step %q: source %d: list source cannot have label", viewName, stepName, index)
		}
		if err := validateTransformCommand(src.Display, viewName, stepName, "display"); err != nil {
			return err
		}
		if err := validateTransformCommand(src.Preview, viewName, stepName, "preview"); err != nil {
			return err
		}
	}

	if hasInput {
		if src.Display != "" {
			return fmt.Errorf("view %q: form step %q: source %d: input source cannot have display", viewName, stepName, index)
		}
		if src.Preview != "" {
			return fmt.Errorf("view %q: form step %q: source %d: input source cannot have preview", viewName, stepName, index)
		}
	}

	return nil
}

func validateTransformCommand(cmd, viewName, stepName, field string) error {
	if cmd == "" {
		return nil
	}
	if strings.Contains(cmd, "{}") || strings.HasPrefix(cmd, "|") {
		return nil
	}
	return fmt.Errorf("view %q: form step %q: %s must contain {} or start with |", viewName, stepName, field)
}
