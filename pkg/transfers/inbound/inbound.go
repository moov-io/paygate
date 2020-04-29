// Copyright 2020 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package inbound

// import (
// 	"github.com/go-kit/kit/metrics/prometheus"
// 	stdprometheus "github.com/prometheus/client_golang/prometheus"
// )

// var (
// 	inboundFilesProcessed = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
// 		Name: "inbound_ach_files_processed",
// 		Help: "Counter of inbound files processed",
// 	}, []string{"origin", "destination"})
// )

// inboundFilesProcessed.With("origin", file.Header.ImmediateOrigin, "destination", file.Header.ImmediateDestination).Add(1)

// setup to read files from remote service and send off as COR/NOC, prenote, or transfer

// // saveRemoteFiles will write all inbound and return ACH files for a given routing number to the specified directory
// func (c *Controller) saveRemoteFiles(agent upload.Agent, dir string) error {
// 	var errors []string

// 	// Download and save inbound files
// 	files, err := agent.GetInboundFiles()
// 	if err != nil {
// 		errors = append(errors, fmt.Sprintf("%T: GetInboundFiles error=%v", agent, err))
// 	}
// 	// TODO(adam): should we move this into GetInboundFiles with an LStat guard?
// 	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, agent.InboundPath())), 0777); err != nil {
// 		errors = append(errors, fmt.Sprintf("%T: inbound MkdirAll error=%v", agent, err))
// 	}
// 	if err := c.writeFiles(files, filepath.Join(dir, agent.InboundPath())); err != nil {
// 		errors = append(errors, fmt.Sprintf("%T: inbound writeFiles error=%v", agent, err))
// 	}
// 	for i := range files {
// 		c.logger.Log("saveRemoteFiles", fmt.Sprintf("%T: copied down inbound file %s", agent, files[i].Filename))

// 		if err := agent.Delete(filepath.Join(agent.InboundPath(), files[i].Filename)); err != nil {
// 			errors = append(errors, fmt.Sprintf("%T: inbound Delete filename=%s error=%v", agent, files[i].Filename, err))
// 		}
// 	}

// 	// Download and save returned files
// 	files, err = agent.GetReturnFiles()
// 	if err != nil {
// 		errors = append(errors, fmt.Sprintf("%T: GetReturnFiles error=%v", agent, err))
// 	}
// 	// TODO(adam): should we move this into GetReturnFiles with an LStat guard?
// 	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, agent.ReturnPath())), 0777); err != nil {
// 		errors = append(errors, fmt.Sprintf("%T: return MkdirAll error=%v", agent, err))
// 	}
// 	if err := c.writeFiles(files, filepath.Join(dir, agent.ReturnPath())); err != nil {
// 		errors = append(errors, fmt.Sprintf("%T: return writeFiles error=%v", agent, err))
// 	}
// 	for i := range files {
// 		c.logger.Log("saveRemoteFiles", fmt.Sprintf("%T: copied down return file %s", agent, files[i].Filename))

// 		if err := agent.Delete(filepath.Join(agent.ReturnPath(), files[i].Filename)); err != nil {
// 			errors = append(errors, fmt.Sprintf("%T: return Delete filename=%s error=%v", agent, files[i].Filename, err))
// 		}
// 	}

// 	if len(errors) > 0 {
// 		return fmt.Errorf("  " + strings.Join(errors, "\n  "))
// 	}
// 	return nil
// }
