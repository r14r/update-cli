package updater

import (
	"fmt"
	"strings"
)

var publicCommands = map[string]struct{}{
	"help": {}, "howto": {}, "version": {}, "check": {}, "update": {},
	"backup": {}, "history": {}, "cleanup": {},
	"clean": {}, "init": {}, "upgrade": {}, "migrate": {}, "whatsnew": {}, "recover": {}, "doctor": {}, "fix": {}, "status": {},
	"releases": {}, "verify": {}, "unlock": {}, "run": {}, "install": {},
	"bootstrap": {}, "build": {}, "test": {}, "precommit": {}, "ci": {}, "workflow": {},
	"schema": {}, "setup": {}, "config": {}, "templates": {}, "git": {}, "generate-alias": {}, "show-requirements": {}, "generate-config": {},
	"convert-yaml": {}, "create-yaml": {}, "create-setup-script": {},
}

var publicValueParameters = map[string]struct{}{
	"--archive": {}, "-a": {}, "--downloads": {}, "-d": {}, "--mode": {},
	"--from": {}, "--folder": {}, "--url": {}, "--repository": {}, "--repo": {},
	"--from-repository": {}, "--root": {}, "-r": {}, "--keep": {}, "--limit": {},
	"--use-template": {}, "--use": {}, "--manifest": {}, "--task": {},
	"--workflow": {}, "--command": {}, "--set": {}, "--save": {},
	"--project": {}, "--activation": {},
}

// ValidatePublicSyntax enforces the single public CLI grammar:
//
//	update-cli <command> --<parameter> [value] ...
//
// The no-parameter invocation remains valid because it is configured separately.
func ValidatePublicSyntax(args []string) error {
	if len(args) == 0 {
		return nil
	}
	command := strings.ToLower(strings.TrimSpace(args[0]))
	if (command == "generate-alias" || command == "show-requirements" || command == "generate-config") && len(args) != 1 {
		return fmt.Errorf("%s akzeptiert keine Parameter", command)
	}
	if command == "git" {
		return validateGitArguments(args[1:])
	}
	if strings.HasPrefix(command, "-") {
		return fmt.Errorf("das erste Argument muss ein Kommando ohne '--' sein; verwenden Sie z. B. 'update-cli version' statt 'update-cli --version'")
	}
	if _, ok := publicCommands[command]; !ok {
		return fmt.Errorf("unbekanntes Kommando %q; verwenden Sie 'update-cli help'", args[0])
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unerwartetes positionsabhängiges Argument %q; nach dem Kommando sind nur --parameter zulässig", arg)
		}
		name := arg
		if before, _, ok := strings.Cut(arg, "="); ok {
			name = before
			continue
		}
		_, needsValue := publicValueParameters[name]
		if command == "setup" && name == "--run" {
			needsValue = true
		}
		if command == "whatsnew" && name == "--version" {
			needsValue = true
		}
		if needsValue {
			if i+1 >= len(args) {
				return fmt.Errorf("Parameter %s benötigt einen Wert", name)
			}
			i++
		}
	}
	return nil
}
