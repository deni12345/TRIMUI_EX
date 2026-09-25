package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const installJobPath = "/tmp/romsearch-install.json"

type installJob struct {
	Game    Game   `json:"game"`
	RomRoot string `json:"rom_root"`
	ImgRoot string `json:"img_root"`
	State   string `json:"state"`
	Stage   string `json:"stage"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Path    string `json:"path,omitempty"`
	Error   string `json:"error,omitempty"`
	PID     int    `json:"pid"`
	Updated int64  `json:"updated"`
}

func readInstallJob() (*installJob, error) {
	data, err := os.ReadFile(installJobPath)
	if err != nil {
		return nil, err
	}
	var job installJob
	if err = json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func writeInstallJob(job *installJob) error {
	job.Updated = time.Now().UnixNano()
	file, err := os.CreateTemp(filepath.Dir(installJobPath), ".romsearch-job-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Chmod(0600); err == nil {
		err = json.NewEncoder(file).Encode(job)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, installJobPath)
}

func jobActive(job *installJob) bool {
	if job == nil || (job.State != "queued" && job.State != "running") {
		return false
	}
	if job.PID > 0 {
		return syscall.Kill(job.PID, 0) == nil
	}
	return time.Since(time.Unix(0, job.Updated)) < 5*time.Second
}

func startInstallJob(game Game, romRoot, imgRoot string) (*installJob, error) {
	if existing, err := readInstallJob(); err == nil && jobActive(existing) {
		return existing, fmt.Errorf("download already running")
	}
	job := &installJob{Game: game, RomRoot: romRoot, ImgRoot: imgRoot, State: "queued", Stage: "Starting download"}
	if err := writeInstallJob(job); err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile("/tmp/rom-search-worker.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()
	cmd := exec.Command(exe, "--install-worker")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err = cmd.Start(); err != nil {
		job.State, job.Error = "error", err.Error()
		_ = writeInstallJob(job)
		return nil, err
	}
	go func() { _ = cmd.Wait() }()
	return job, nil
}

func runInstallWorker() error {
	job, err := readInstallJob()
	if err != nil {
		return err
	}
	job.PID = os.Getpid()
	job.State, job.Stage = "running", "Finding download"
	if err = writeInstallJob(job); err != nil {
		return err
	}
	lastProgress := time.Time{}
	progress := func(done, total int64) {
		job.Done, job.Total = done, total
		job.Stage = "Downloading"
		if time.Since(lastProgress) >= 250*time.Millisecond || (total > 0 && done == total) {
			_ = writeInstallJob(job)
			lastProgress = time.Now()
		}
	}
	status := func(stage string) {
		job.Stage, job.Done, job.Total = stage, 0, 0
		_ = writeInstallJob(job)
	}
	path, installErr := install(job.Game, job.RomRoot, job.ImgRoot, progress, status)
	job.Path = path
	if installErr != nil {
		job.State, job.Error, job.Stage = "error", installErr.Error(), "Download failed"
	} else {
		job.State, job.Stage = "done", "Installed"
	}
	if err = writeInstallJob(job); err != nil {
		return err
	}
	return installErr
}
