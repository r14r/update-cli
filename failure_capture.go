package main

import (
 "bytes"
 "fmt"
 "io"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
 "sync"
 "time"
)

// captureCommandOutput mirrors stdout/stderr while retaining the complete
// transcript. It saves the transcript only when the command fails.
func captureCommandOutput(run func() error) (error, string, error) {
 originalOut, originalErr := os.Stdout, os.Stderr
 outR, outW, err := os.Pipe()
 if err != nil { return err, "", err }
 errR, errW, err := os.Pipe()
 if err != nil { outR.Close(); outW.Close(); return err, "", err }
 var mu sync.Mutex
 var transcript bytes.Buffer
 drain := func(r *os.File, target *os.File, done *sync.WaitGroup) {
  defer done.Done()
  defer r.Close()
  buf := make([]byte, 32*1024)
  for {
   n, e := r.Read(buf)
   if n > 0 {
    _, _ = target.Write(buf[:n])
    mu.Lock()
    _, _ = transcript.Write(buf[:n])
    mu.Unlock()
   }
   if e != nil { return }
  }
 }
 var wg sync.WaitGroup
 wg.Add(2)
 go drain(outR, originalOut, &wg)
 go drain(errR, originalErr, &wg)
 os.Stdout, os.Stderr = outW, errW
 var result error
 func() {
  defer func() {
   if recovered := recover(); recovered != nil {
    result = fmt.Errorf("panic: %v", recovered)
   }
  }()
  result = run()
 }()
 os.Stdout, os.Stderr = originalOut, originalErr
 _ = outW.Close()
 _ = errW.Close()
 wg.Wait()
 if result == nil { return nil, "", nil }
 mu.Lock()
 text := transcript.String()
 mu.Unlock()
 text += "\nERROR  " + result.Error() + "\n"
 directory, e := os.UserCacheDir()
 if e != nil { directory = os.TempDir() }
 directory = filepath.Join(directory, "update-cli", "logs")
 if e := os.MkdirAll(directory, 0o700); e != nil { return result, "", e }
 path := filepath.Join(directory, "failure-"+time.Now().Format("20060102-150405.000000000")+".log")
 if e := os.WriteFile(path, []byte(text), 0o600); e != nil { return result, "", e }
 return result, path, nil
}

// askCopyFailureLog presents a compact terminal dialog only for interactive
// sessions. Background jobs retain the logfile without an interactive pause.
func askCopyFailureLog(path string) {
 if path == "" { return }
 fmt.Fprintf(os.Stderr, "\nFailure log saved: %s\n", path)
 if !isInteractiveInput() { return }
 fmt.Fprintln(os.Stderr, "+-------------------------------------------------------+")
 fmt.Fprintln(os.Stderr, "| Fehler: Copy Error Message to Clipboard?             |")
 fmt.Fprintln(os.Stderr, "+-------------------------------------------------------+")
 fmt.Fprint(os.Stderr, "Copy Error Message to Clipboard? [y/N]: ")
 var answer string
 _, _ = fmt.Fscanln(os.Stdin, &answer)
 switch strings.ToLower(strings.TrimSpace(answer)) {
 case "y", "yes", "j", "ja":
 default:
  return
 }
 b, err := os.ReadFile(path)
 if err != nil { fmt.Fprintln(os.Stderr, "Cannot read log:", err); return }
 if err := copyErrorLog(b); err != nil {
  fmt.Fprintln(os.Stderr, "Clipboard unavailable:", err)
  return
 }
 fmt.Fprintln(os.Stderr, "Error log copied to clipboard.")
}

func isInteractiveInput() bool {
 fi, err := os.Stdin.Stat()
 return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func copyErrorLog(b []byte) error {
 choices := [][]string{}
 switch runtime.GOOS {
 case "darwin":
  choices = append(choices, []string{"pbcopy"})
 case "windows":
  choices = append(choices, []string{"clip"})
 default:
  choices = append(choices, []string{"wl-copy"}, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
 }
 for _, args := range choices {
  path, err := exec.LookPath(args[0])
  if err != nil { continue }
  cmd := exec.Command(path, args[1:]...)
  cmd.Stdin = bytes.NewReader(b)
  if err := cmd.Run(); err == nil { return nil }
 }
 return fmt.Errorf("no supported clipboard utility was found")
}

func shouldRecordFailure(args []string) bool {
 if len(args)==0 { return true }
 for _, arg := range args {
  if arg=="--no-ui" || arg=="--noui" { continue }
 }
 cmd := strings.TrimLeft(strings.ToLower(args[0]), "-")
 return cmd=="update" || cmd=="setup" || cmd=="bootstrap" || cmd=="build" || cmd=="install"
}

var _ io.Reader = (*bytes.Buffer)(nil)
