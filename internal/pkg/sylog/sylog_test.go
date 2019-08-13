// Copyright (c) 2018-2019, Sylabs Inc. All rights reserved.
// This software is licensed under a 3-clause BSD license. Please consult the
// LICENSE.md file distributed with the sources of this project regarding your
// rights to use or distribute this software.

// +build sylog

package sylog

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/sylabs/singularity/internal/pkg/test"
)

func TestPrefix(t *testing.T) {
	// This is information necessary to deal with the string generated by
	// prefix() in the context of Debug mode. This is because in debug mode
	// we display information about the context, i.e., PIDs, function caller.
	// Note that we execute these functions before we drop privileges to make
	// sure we can the UID in a way that is compliant to the Sylog code.
	uid := os.Geteuid()
	pid := os.Getpid()
	uidStr := fmt.Sprintf("[U=%d,P=%d]", uid, pid)
	funcName := "goexit()"

	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tests := []struct {
		name     string
		lvl      messageLevel
		msgColor string
		levelStr string
	}{
		{
			name:     "invalid",
			lvl:      messageLevel(fatal - 1),
			msgColor: "\x1b[0m",
			levelStr: "????",
		},
		{
			name:     "fatal",
			lvl:      fatal,
			msgColor: "\x1b[31m",
			levelStr: "FATAL",
		},
		{
			name:     "error",
			lvl:      error,
			msgColor: "\x1b[31m",
			levelStr: "ERROR",
		},
		{
			name:     "warn",
			lvl:      warn,
			msgColor: "\x1b[33m",
			levelStr: "WARNING",
		},
		{
			name:     "info",
			lvl:      info,
			msgColor: "\x1b[34m",
			levelStr: "INFO",
		},
		{
			name:     "debug",
			lvl:      debug,
			msgColor: "\x1b[0m",
			levelStr: "DEBUG",
		},
	}

	// With color
	for _, tt := range tests {
		t.Run("color_"+tt.name, func(t *testing.T) {
			SetLevel(int(tt.lvl)) // This impacts the output format
			p := prefix(tt.lvl)
			expectedOutput := fmt.Sprintf("%s%-8s%s ", tt.msgColor, tt.levelStr+":", "\x1b[0m")
			if tt.name == "debug" {
				expectedOutput = fmt.Sprintf("%s%-8s%s%-19s%-30s", tt.msgColor, tt.lvl, "\x1b[0m", uidStr, funcName)
			}
			if p != expectedOutput {
				t.Fatalf("test returned %s. instead of %s.", p, expectedOutput)
			}
		})
	}

	// Without color
	for _, tt := range tests {
		t.Run("nocolor_"+tt.name, func(t *testing.T) {
			DisableColor()
			SetLevel(int(tt.lvl)) // This impacts the output format
			p := prefix(tt.lvl)
			expectedOutput := fmt.Sprintf("%s%-8s%s ", "", tt.levelStr+":", "")
			// invalid cases do *not* support disabling color
			if tt.name == "invalid" {
				expectedOutput = fmt.Sprintf("%s%-8s%s ", "\x1b[0m", tt.levelStr+":", "")
			}
			// debug is special too and does not support disabling color
			if tt.name == "debug" {
				expectedOutput = fmt.Sprintf("%s%-8s%s%-19s%-30s", tt.msgColor, tt.lvl, "", uidStr, funcName)
			}
			if p != expectedOutput {
				t.Fatalf("test returned %s. instead of %s.", p, expectedOutput)
			}
		})
	}
}

func TestWriter(t *testing.T) {
	test.DropPrivilege(t)
	defer test.ResetPrivilege(t)

	tests := []struct {
		name           string
		loggerLevel    int
		expectedResult io.Writer
	}{
		{
			name:           "undefined level",
			loggerLevel:    int(fatal - 1),
			expectedResult: ioutil.Discard,
		},
		{
			name:           "no logger",
			loggerLevel:    0,
			expectedResult: os.Stderr,
		},
		{
			name:           "valid logger",
			loggerLevel:    1,
			expectedResult: os.Stderr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(tt.loggerLevel)
			w := Writer()
			if w != tt.expectedResult {
				if w == ioutil.Discard {
					fmt.Printf("%s returned ioutil.Discard\n", tt.name)
				}
				if w == os.Stderr {
					fmt.Printf("%s returned os.Stderr\n", tt.name)
				}
				t.Fatal("Writer() did not return the expected io.Writer")
			}
		})
	}
}

func TestWritef(t *testing.T) {
	const str = "just a test"

	tests := []struct {
		name string
		lvl  messageLevel
	}{
		{
			name: "info",
			lvl:  info,
		},
		{
			name: "error",
			lvl:  error,
		},
		{
			name: "warning",
			lvl:  warn,
		},
		{
			name: "fatal",
			lvl:  fatal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			SetLevel(int(tt.lvl))
			DisableColor()

			writef(&buf, tt.lvl, "%s", str)
			expectedResult := prefix(tt.lvl) + str + "\n"
			if buf.String() != expectedResult {
				t.Fatalf("test %s returned %s instead of %s", tt.name, buf.String(), expectedResult)
			}
		})
	}

	// corner case
	SetLevel(int(fatal))
	expectedResult := ""
	var buf bytes.Buffer
	writef(&buf, info, "%s", str)
	if buf.String() != expectedResult {
		t.Fatalf("test returned %s instead of an empty string", buf.String())
	}
}

func TestGetLevel(t *testing.T) {
	tests := []struct {
		name           string
		lvl            messageLevel
		expectedResult int
	}{
		{
			name:           "fatal",
			lvl:            fatal,
			expectedResult: -4,
		},
		{
			name:           "error",
			lvl:            error,
			expectedResult: -3,
		},
		{
			name:           "warn",
			lvl:            warn,
			expectedResult: -2,
		},
		{
			name:           "info",
			lvl:            info,
			expectedResult: 1,
		},
		{
			name:           "verbose",
			lvl:            verbose,
			expectedResult: 2,
		},
		{
			name:           "verbose2",
			lvl:            verbose2,
			expectedResult: 3,
		},
		{
			name:           "verbose3",
			lvl:            verbose3,
			expectedResult: 4,
		},
		{
			name:           "debug",
			lvl:            debug,
			expectedResult: 5,
		},
		{
			name:           "invalid",
			lvl:            messageLevel(-10),
			expectedResult: -4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLevel(int(tt.lvl))
			lvl := GetLevel()
			if lvl != int(tt.lvl) {
				t.Fatalf("test %s was expected to return %d but returned %d instead", tt.name, tt.expectedResult, lvl)
			}
		})
	}
}

func TestGetenv(t *testing.T) {
	str := GetEnvVar()
	expectedResult := "SINGULARITY_MESSAGELEVEL="
	if str[:25] != expectedResult {
		t.Fatalf("Test returned %s instead of %s", str[:25], expectedResult)
	}
}

const testStr = "test message"

type fnOut func(format string, a ...interface{})

func runTestLogFn(t *testing.T, errFd *os.File, fn fnOut) {

	if errFd != nil {
		fn("%s", testStr)
		return
	}

	SetLevel(int(debug))

	rescueStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %s", err)
	}
	os.Stderr = w

	fn("%s\n", testStr)

	w.Close() // This will enable the read operation
	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read from pipe: %s", err)
	}
	os.Stderr = rescueStderr

	// We check the formatting of the output we caught
	regExpClass := regexp.MustCompile(`^(.*) \[U=`)
	classResult := regExpClass.FindStringSubmatch(string(out))
	if len(classResult) < 2 {
		t.Fatalf("unexpected format: %s", string(out))
	}
	class := classResult[1]
	class = strings.Trim(class, " \t")
	if class != "WARNING" && class != "INFO" && class != "\x1b[0mDEBUG" && class != "\x1b[0mVERBOSE" {
		t.Fatalf("failed to recognize the type of message: %s.", class)
	}

	regExpMsg := regexp.MustCompile(`runTestLogFn\(\)(.*)\n`)
	msgResult := regExpMsg.FindStringSubmatch(string(out))
	if len(msgResult) < 2 {
		t.Fatalf("unexpected format: %s", string(out))
	}
	msg := msgResult[1]
	if msg[len(msg)-len(testStr):] != testStr {
		t.Fatalf("invalid test message: %s vs. %s", msg[len(msg)-len(testStr):], testStr)
	}
}

func TestStderrOutput(t *testing.T) {

	tests := []struct {
		name string
		out  *os.File
	}{
		{
			// We just call a few funtions that output to stderr, not much we can test
			// except make sure that whatever potential modification to the code does
			// not make the code crash
			name: "default Stderr",
			out:  os.Stderr,
		},
		{
			name: "pipe",
			out:  nil, // Since nil, the code will create a pipe for that case so we can catch what is written to stderr
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runTestLogFn(t, tt.out, Warningf)
			runTestLogFn(t, tt.out, Infof)
			runTestLogFn(t, tt.out, Verbosef)
			runTestLogFn(t, tt.out, Debugf)
		})
	}
}