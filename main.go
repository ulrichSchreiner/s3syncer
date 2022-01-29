package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/inconshreveable/log15"
	cron "github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

const (
	triggerPath = "/trigger/"
	healthyPath = "/health"

	evPUSH = "push"
)

var (
	flagConfig = flag.String("config", "/etc/s3syncer/config.yaml", "set the configuration path")
	flagListen = flag.String("listen", "localhost:9999", "the address to listen")
	config     *Config
)

type RegistryEvent struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Target    struct {
		MediaType  string `json:"mediaType"`
		Size       int64  `json:"size"`
		Digest     string `json:"digest"`
		Length     int64  `json:"length"`
		Repository string `json:"repository"`
		URL        string `json:"url"`
		Tag        string `json:"tag"`
	} `json:"target"`
	Request struct {
		ID        string `json:"id"`
		Addr      string `json:"addr"`
		Host      string `json:"host"`
		Method    string `json:"method"`
		Useragent string `json:"useragent"`
	} `json:"request"`
	Source struct {
		Addr       string `json:"addr"`
		InstanceID string `json:"instanceID"`
	} `json:"source"`
}

type EventEnvelope struct {
	Events []RegistryEvent `json:"events"`
}

type TriggerCommand struct {
	Name       string            `yaml:"name"`
	Cmd        string            `yaml:"cmd"`
	Workdir    string            `yaml:"workdir"`
	Args       []string          `yaml:"args"`
	Env        map[string]string `yaml:"env"`
	Delay      time.Duration     `yaml:"delay"`
	Reconcile  time.Duration     `yaml:"reconcile"`
	RunAt      string            `yaml:"runAt"`
	RunOnStart bool              `yaml:"runOnStart"`
	Disable    bool              `yaml:"disable"`
}

type commandQueue chan bool
type DebouncedCommand struct {
	triggerQueue commandQueue
	Cmd          TriggerCommand
}

type Config struct {
	Cmds []TriggerCommand `yaml:"commands"`

	commands map[string]*DebouncedCommand
}

func (dbc *DebouncedCommand) execCommand() {
	if dbc.Cmd.Disable {
		log15.Info("command disabled", "name", dbc.Cmd.Name)
		return
	}

	log15.Info("exec command", "command", dbc.Cmd, "name", dbc.Cmd.Name)
	c := exec.Command(dbc.Cmd.Cmd, dbc.Cmd.Args...)
	environ := os.Environ()
	for k, v := range dbc.Cmd.Env {
		environ = append(environ, fmt.Sprintf("%s=%s", k, v))
	}
	c.Env = environ
	if dbc.Cmd.Workdir != "" {
		c.Dir = dbc.Cmd.Workdir
	}
	t := time.Now()
	out, err := c.Output()
	d := time.Since(t)
	if err != nil {
		log15.Error("command failed", "name", dbc.Cmd.Name, "duration", d, "cmd", dbc.Cmd.Cmd, "args", dbc.Cmd.Args, "environment", environ, "output", string(out), "error", err)
	} else {
		log15.Info("command executed", "name", dbc.Cmd.Name, "duration", d, "cmd", dbc.Cmd.Cmd, "args", dbc.Cmd.Args, "environment", environ)
	}
}

func (cfg *Config) StartServices() {
	for _, c := range cfg.commands {
		go c.debounce()
	}
}

func (dbc *DebouncedCommand) trigger() {
	go func() {
		dbc.triggerQueue <- true
	}()
}

func (dbc *DebouncedCommand) debounce() {
	if dbc.Cmd.RunOnStart {
		dbc.execCommand()
	}
	if dbc.Cmd.RunAt != "" {
		c := cron.New()
		_, err := c.AddFunc(dbc.Cmd.RunAt, dbc.trigger)
		if err != nil {
			log15.Error("cannot add cron", "error", err)
		} else {
			c.Start()
		}
	}
	ok := false
	timer := time.NewTimer(dbc.Cmd.Delay)
	rec := time.NewTimer(dbc.Cmd.Reconcile)

	for {
		select {
		case ok = <-dbc.triggerQueue:
			timer.Reset(dbc.Cmd.Delay)
		case <-rec.C:
			dbc.execCommand()
			ok = false
			rec.Reset(dbc.Cmd.Reconcile)
		case <-timer.C:
			if ok {
				dbc.execCommand()
				ok = false
			}
		}
	}

}

func trigger(w http.ResponseWriter, r *http.Request) {
	cmd := r.URL.Path[len(triggerPath):]
	var envelope EventEnvelope
	err := json.NewDecoder(r.Body).Decode(&envelope)
	if err != nil {
		log15.Error("cannot decode event", "path", r.URL.Path, "name", cmd, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	j, _ := json.Marshal(envelope)
	if c, ok := config.commands[cmd]; ok {
		if len(envelope.Events) > 0 {
			for _, ev := range envelope.Events {
				if ev.Action == evPUSH && ev.Target.Tag != "" {
					log15.Info("new trigger request", "path", r.URL.Path, "name", cmd, "envelope", envelope, "jdata", string(j))
					c.trigger()
				}
			}
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, http.StatusText(http.StatusOK))
		return
	}
	log15.Error("illegal request", "path", r.URL.Path, "name", cmd, "envelope", envelope)
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, http.StatusText(http.StatusNotFound))
}

func healthy(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, http.StatusText(http.StatusOK))
}

func main() {
	flag.Parse()
	cfgfile, err := os.Open(*flagConfig)
	if err != nil {
		log15.Error("cannot open config", "path", *flagConfig, "error", err)
		os.Exit(1)
	}
	config, err = loadConfig(cfgfile)
	if err != nil {
		log15.Error("cannot parse config", "path", *flagConfig, "error", err)
		os.Exit(1)
	}
	_ = cfgfile.Close()

	config.StartServices()

	http.HandleFunc(triggerPath, trigger)
	http.HandleFunc(healthyPath, healthy)

	log15.Info("start service", "address", *flagListen, "config", *config)
	_ = http.ListenAndServe(*flagListen, nil)
}

func loadConfig(config io.Reader) (*Config, error) {
	c := &Config{
		Cmds:     []TriggerCommand{},
		commands: make(map[string]*DebouncedCommand),
	}
	err := yaml.NewDecoder(config).Decode(c)
	if err != nil {
		return nil, err
	}
	for _, cmd := range c.Cmds {
		if cmd.Delay == 0 {
			cmd.Delay = 5 * time.Second
		}
		if cmd.Reconcile == 0 {
			cmd.Reconcile = 24 * time.Hour * 365 * 10
		}
		c.commands[cmd.Name] = &DebouncedCommand{
			triggerQueue: make(commandQueue),
			Cmd:          cmd,
		}
	}
	return c, nil
}
