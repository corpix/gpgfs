package cli

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	revip "github.com/corpix/revip"
	spew "github.com/davecgh/go-spew/spew"
	cli "github.com/urfave/cli/v2"
	di "go.uber.org/dig"
	daemon "github.com/coreos/go-systemd/daemon"

	"git.backbone/corpix/gpgfs/pkg/bus"
	"git.backbone/corpix/gpgfs/pkg/config"
	"git.backbone/corpix/gpgfs/pkg/crypto"
	"git.backbone/corpix/gpgfs/pkg/errors"
	"git.backbone/corpix/gpgfs/pkg/fuse"
	"git.backbone/corpix/gpgfs/pkg/log"
	"git.backbone/corpix/gpgfs/pkg/meta"
	"git.backbone/corpix/gpgfs/pkg/telemetry"
)

var (
	Stdout = os.Stdout
	Stderr = os.Stderr

	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "log-level",
			Aliases: []string{"l"},
			Usage:   "logging level (debug, info, warn, error)",
		},
		&cli.StringSliceFlag{
			Name:    "config",
			Aliases: []string{"c"},
			EnvVars: []string{config.EnvironPrefix + "_CONFIG"},
			Usage:   "path to application configuration file/files (separate multiple files with comma)",
			Value:   cli.NewStringSlice("config.yml"),
		},

		//

		&cli.DurationFlag{
			Name:  "duration",
			Usage: "exit after duration",
		},
		&cli.BoolFlag{
			Name:  "profile",
			Usage: "write profile information for debugging (cpu.prof, heap.prof)",
		},
		&cli.BoolFlag{
			Name:  "trace",
			Usage: "write trace information for debugging (trace.prof)",
		},
	}
	Commands = []*cli.Command{
		{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Configuration tools",
			Subcommands: []*cli.Command{
				{
					Name:    "show-default",
					Aliases: []string{"sd"},
					Usage:   "Show default configuration",
					Action:  ConfigShowDefaultAction,
				},
				{
					Name:    "show",
					Aliases: []string{"s"},
					Usage:   "Show default configuration",
					Action:  ConfigShowAction,
				},
				{
					Name:    "validate",
					Aliases: []string{"v"},
					Usage:   "Validate configuration and exit",
					Action:  ConfigValidateAction,
				},
				{
					Name:      "push",
					Aliases:   []string{"p"},
					Usage:     "Push configuration to specified destination",
					Action:    ConfigPushAction,
					ArgsUsage: "<destination>[,...]",
				},
			},
		},
		{
			Name:    "key",
			Aliases: []string{"k"},
			Usage:   "Key tooling",
			Subcommands: []*cli.Command{
				{
					Name:    "convert",
					Aliases: []string{"c"},
					Usage:   "Convert key from one format into another",
					Action:  KeyConvertAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "type",
							Aliases: []string{"t"},
							Value:   "public",
							Usage:   "Key type to output (public or private)",
						},
						&cli.StringFlag{
							Name:    "input",
							Aliases: []string{"i"},
							Value:   "-",
							Usage:   "Key file or '-' to use stdin as a source to read key",
						},
						&cli.StringFlag{
							Name:    "output",
							Aliases: []string{"o"},
							Value:   "-",
							Usage:   "Key file or '-' to use stdout as a target to write key",
						},
					},
				},
			},
		},
		{
			Name:    "message",
			Aliases: []string{"m"},
			Usage:   "Message tooling",
			Subcommands: []*cli.Command{
				{
					Name:    "encrypt",
					Aliases: []string{"e"},
					Usage:   "Encrypt message",
					Action:  MessageEncryptAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "key",
							Aliases:  []string{"k"},
							Required: true,
							Usage:    "Key path on the filesystem (private)",
						},
						&cli.StringFlag{
							Name:    "input",
							Aliases: []string{"i"},
							Value:   "-",
							Usage:   "Key file or '-' to use stdin as a source to read key",
						},
						&cli.StringFlag{
							Name:    "output",
							Aliases: []string{"o"},
							Value:   "-",
							Usage:   "Key file or '-' to use stdout as a target to write key",
						},
					},
				},
				{
					Name:    "decrypt",
					Aliases: []string{"e"},
					Usage:   "Decrypt message",
					Action:  MessageDecryptAction,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "key",
							Aliases:  []string{"k"},
							Required: true,
							Usage:    "Key path on the filesystem (private)",
						},
						&cli.StringFlag{
							Name:    "input",
							Aliases: []string{"i"},
							Value:   "-",
							Usage:   "Key file or '-' to use stdin as a source to read key",
						},
						&cli.StringFlag{
							Name:    "output",
							Aliases: []string{"o"},
							Value:   "-",
							Usage:   "Key file or '-' to use stdout as a target to write key",
						},
					},
				},
			},
		},
		{
			Name:    "mount",
			Aliases: []string{"m"},
			Usage:   "Mount GPG FUSE",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "source",
					Aliases:  []string{"s"},
					Usage:    "source directory with .gpg tree",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "target",
					Aliases:  []string{"t"},
					Usage:    "target directory to mount filesystem with decrypted files",
					Required: true,
				},
			},
			Action: MountAction,
		},
	}

	c *di.Container
)

type doneCh = chan struct{}

func Before(ctx *cli.Context) error {
	var err error

	c = di.New()

	//

	err = c.Provide(func() doneCh { return make(doneCh) })
	if err != nil {
		return err
	}

	err = c.Provide(func() *cli.Context { return ctx })
	if err != nil {
		return err
	}

	err = c.Provide(func() *spew.ConfigState {
		return &spew.ConfigState{
			DisableMethods:          false,
			DisableCapacities:       true,
			DisablePointerAddresses: true,
			Indent:                  "  ",
			SortKeys:                true,
			SpewKeys:                false,
		}
	})
	if err != nil {
		return err
	}

	err = c.Provide(func() *json.Encoder {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc
	})
	if err != nil {
		return err
	}

	err = c.Provide(func(ctx *cli.Context) (*config.Config, error) {
		c, err := config.Load(ctx.StringSlice("config"))
		if err != nil {
			return nil, err
		}

		return c, nil
	})
	if err != nil {
		return err
	}

	err = c.Provide(func(ctx *cli.Context, c *config.Config) (log.Logger, error) {
		lc := *c.Log
		level := ctx.String("log-level")
		if level != "" {
			lc.Level = level
		}

		return log.Create(lc)
	})
	if err != nil {
		return err
	}

	err = c.Provide(func() crypto.Rand { return crypto.DefaultRand })
	if err != nil {
		return err
	}
	err = c.Provide(func() *telemetry.Registry { return telemetry.DefaultRegistry })
	if err != nil {
		return err
	}

	//

	err = c.Provide(func(
		c *config.Config,
		l log.Logger,
		r *telemetry.Registry,
		done doneCh,
		running *sync.WaitGroup,
		errc chan error,
	) (*telemetry.Server, error) {
		start := func(t *telemetry.Server) {
			errc <- errors.Wrap(
				t.ListenAndServe(),
				"failed while listen and serve telemetry server",
			)
		}

		finalize := func(t *telemetry.Server) {
			defer running.Done()

			<-done
			err = t.Shutdown(context.Background())
			if err != nil {
				panic(errors.Wrap(err, "telemetry shutdown failed"))
			}
		}

		if c.Telemetry.Enable {
			lr, err := net.Listen("tcp", c.Telemetry.Addr)
			if err != nil {
				return nil, err
			}
			t := telemetry.New(*c.Telemetry, l, r, lr)

			running.Add(1)

			go start(t)
			go finalize(t)

			return t, nil
		}

		return nil, nil
	})
	if err != nil {
		return err
	}

	//

	err = c.Provide(func() *sync.WaitGroup { return &sync.WaitGroup{} })
	if err != nil {
		return err
	}

	err = c.Provide(func() chan error { return make(chan error, 1) })
	if err != nil {
		return err
	}

	err = c.Provide(func() chan os.Signal {
		sig := make(chan os.Signal, 1)
		signal.Notify(
			sig,
			syscall.SIGQUIT,
			syscall.SIGTERM,
			syscall.SIGINT,
			syscall.SIGUSR1,
			syscall.SIGUSR2,
			syscall.SIGHUP,
		)
		return sig
	})
	if err != nil {
		return err
	}

	//

	duration := ctx.Duration("duration")
	if duration == 0 {
		err = c.Provide(func(ctx *cli.Context) context.Context {
			return context.Background()
		})
	} else {
		err = c.Provide(func(ctx *cli.Context) context.Context {
			c, cancel := context.WithTimeout(context.Background(), duration)
			go func() {
				<-c.Done()
				cancel()
			}()
			return c
		})
	}
	if err != nil {
		return err
	}

	//

	if ctx.Bool("profile") {
		err = c.Invoke(writeProfile)
		if err != nil {
			return err
		}
	}

	if ctx.Bool("trace") {
		err = c.Invoke(writeTrace)
		if err != nil {
			return err
		}
	}

	return nil
}

//

func ConfigShowDefaultAction(ctx *cli.Context) error {
	c, err := config.Default()
	if err != nil {
		return err
	}

	write := revip.ToWriter(os.Stdout, config.Marshaler)

	return write(c)
}

func ConfigShowAction(ctx *cli.Context) error {
	return c.Invoke(func(c *config.Config) error {
		write := revip.ToWriter(os.Stdout, config.Marshaler)

		return write(c)
	})
}

func ConfigValidateAction(ctx *cli.Context) error {
	return c.Invoke(func(l log.Logger) error {
		configs := ctx.StringSlice("config")
		c, err := config.Load(
			configs,
			config.InitPostprocessors...,
		)
		if err != nil {
			return err
		}

		err = config.Validate(c)
		if err != nil {
			return err
		}

		l.Info().
			Strs("configs", configs).
			Msg("configuration validation is ok")

		return nil
	})
}

func ConfigPushAction(ctx *cli.Context) error {
	return c.Invoke(func(l log.Logger) error {
		configs := ctx.StringSlice("config")
		c, err := config.Load(
			configs,
			config.LocalPostprocessors...,
		)
		if err != nil {
			return err
		}

		args := ctx.Args().Slice()
		if len(args) < 1 {
			return errors.New("subcommand requires an argument, example: ./config.out.yml")
		}

		destinations := args
		for _, destination := range destinations {
			push, err := revip.ToURL(destination, config.Marshaler)
			if err != nil {
				return err
			}

			err = push(c)
			if err != nil {
				return err
			}
		}

		l.Info().
			Strs("configs", configs).
			Strs("destinations", destinations).
			Msg("configuration pushed")

		return nil
	})
}

//

func KeyConvertAction(ctx *cli.Context) error {
	return c.Invoke(func() error {
		var (
			input   io.ReadCloser
			output  io.WriteCloser
			err     error
			keyType = ctx.String("type")
		)

		//

		inputName := ctx.String("input")
		if inputName == "-" {
			input = os.Stdin
		} else {
			input, err = os.Open(inputName)
			if err != nil {
				return err
			}
			defer input.Close()
		}

		outputName := ctx.String("output")
		if outputName == "-" {
			output = os.Stdout
		} else {
			output, err = os.OpenFile(
				outputName,
				os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
				0600,
			)
			if err != nil {
				return err
			}
			defer output.Close()
		}

		//

		chain, err := ioutil.ReadAll(input)
		if err != nil {
			return err
		}

	next:
		block, rest := pem.Decode(chain)
		if block == nil {
			return nil
		}
		chain = rest

		enclave, err := fuse.NewKey(
			fuse.KeyFormatSSH,
			fuse.DefaultKeyUID,
			keyType,
			pem.EncodeToMemory(block),
		)
		if err != nil {
			return err
		}
		buf, err := enclave.Open()
		if err != nil {
			return err
		}
		defer buf.Destroy()

		fmt.Fprint(output, string(buf.Bytes()))
		fmt.Fprint(output, "\n")

		goto next
	})
}

func MessageEncryptAction(ctx *cli.Context) error {
	return c.Invoke(func() error {
		var (
			input  io.ReadCloser
			output io.WriteCloser
			err    error
			key    = ctx.String("key")
		)

		//

		inputName := ctx.String("input")
		if inputName == "-" {
			input = os.Stdin
		} else {
			input, err = os.Open(inputName)
			if err != nil {
				return err
			}
			defer input.Close()
		}

		outputName := ctx.String("output")
		if outputName == "-" {
			output = os.Stdout
		} else {
			output, err = os.OpenFile(
				outputName,
				os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
				0600,
			)
			if err != nil {
				return err
			}
			defer output.Close()
		}

		//

		rawKey, err := os.ReadFile(key)
		if err != nil {
			return err
		}

		enclave, err := fuse.NewKey(
			fuse.KeyFormatSSH,
			fuse.DefaultKeyUID,
			fuse.KeyTypePrivate,
			rawKey,
		)
		if err != nil {
			return err
		}
		buf, err := enclave.Open()
		if err != nil {
			return err
		}
		defer buf.Destroy()

		msg, err := ioutil.ReadAll(input)
		if err != nil {
			return err
		}

		encBuf, err := fuse.Encrypt(buf, fuse.NewPlainMessage(msg))
		if err != nil {
			return err
		}

		_, err = output.Write(encBuf)
		return err
	})
}

func MessageDecryptAction(ctx *cli.Context) error {
	return c.Invoke(func() error {
		var (
			input  io.ReadCloser
			output io.WriteCloser
			err    error
			key    = ctx.String("key")
		)

		//

		inputName := ctx.String("input")
		if inputName == "-" {
			input = os.Stdin
		} else {
			input, err = os.Open(inputName)
			if err != nil {
				return err
			}
			defer input.Close()
		}

		outputName := ctx.String("output")
		if outputName == "-" {
			output = os.Stdout
		} else {
			output, err = os.OpenFile(
				outputName,
				os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
				0600,
			)
			if err != nil {
				return err
			}
			defer output.Close()
		}

		//

		rawKey, err := os.ReadFile(key)
		if err != nil {
			return err
		}

		enclave, err := fuse.NewKey(
			fuse.KeyFormatSSH,
			fuse.DefaultKeyUID,
			fuse.KeyTypePrivate,
			rawKey,
		)
		if err != nil {
			return err
		}
		buf, err := enclave.Open()
		if err != nil {
			return err
		}
		defer buf.Destroy()

		encBuf, err := ioutil.ReadAll(input)
		if err != nil {
			return err
		}

		plainMessage, err := fuse.Decrypt(buf, encBuf)
		if err != nil {
			return err
		}
		defer fuse.WipeBytes(plainMessage.Data)

		_, err = output.Write(plainMessage.Data)
		return err
	})
}

//

func MountAction(ctx *cli.Context) error {
	err := c.Provide(func(
		c *config.Config,
		l log.Logger,
		r *telemetry.Registry,
	) (*fuse.Fuse, error) {
		buf, err := os.ReadFile(c.Fuse.Key.Path)
		if err != nil {
			return nil, err
		}

		enclave, err := fuse.NewKey(
			c.Fuse.Key.Format,
			fuse.DefaultKeyUID,
			fuse.KeyTypePrivate,
			buf,
		)
		if err != nil {
			return nil, err
		}

		return fuse.New(
			*c.Fuse, l,
			enclave,
			ctx.String("source"),
			ctx.String("target"),
		)
	})
	if err != nil {
		return err
	}

	err = c.Provide(func(
		c *config.Config,
		l log.Logger,
		f *fuse.Fuse,
		ctx *cli.Context,
		done doneCh,
		running *sync.WaitGroup,
	) (*fuse.Server, error) {
		s, err := f.Mount()
		if err != nil {
			return nil, err
		}
		err = f.Preload(context.Background())
		if err != nil {
			return nil, err
		}

		go func() {
			defer running.Done()

			<-done
			l.Info().Msg("unmounting")
			err := s.Unmount()
			if err != nil {
				l.Error().Err(err).Msg("failed to unmount fuse")
			}
		}()

		//

		notified, err := daemon.SdNotify(true, daemon.SdNotifyReady)
		if err != nil {
			return nil, err
		}
		if notified {
			l.Debug().Msg("indicated readiness to systemd")
		}

		//

		running.Add(1)

		return s, nil
	})
	if err != nil {
		return err
	}

	//

	components := c.String()
	_ = c.Invoke(func(l log.Logger) {
		l.Trace().Msgf(
			"component graph: %s",
			strings.TrimSpace(components),
		)
	})

	return c.Invoke(func(
		ctx context.Context,
		cfg *config.Config,
		l log.Logger,
		t *telemetry.Server,
		s *fuse.Server,
		running *sync.WaitGroup,
		done doneCh,
		errc chan error,
		sig chan os.Signal,
	) error {
		l.Info().Msg("mounting")

	loop:
		for {
			select {
			case <-ctx.Done():
				break loop
			case err := <-errc:
				if err != nil {
					return err
				}
			case si := <-sig:
				l.Info().Str("signal", si.String()).Msg("received signal")
				switch si {
				case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
					close(done)
					break loop
				case syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGHUP:
				}
			case <-bus.Config:
				// ignore configuration updates at the moment
			}
		}

		//

		defer os.Exit(0)

		time.AfterFunc(cfg.ShutdownGraceTime, func() {
			l.Warn().
				Dur("graceTime", cfg.ShutdownGraceTime).
				Msg("graceful shutdown timed out")
			os.Exit(1)
		})

		running.Wait() // wait for other running components to finish

		return nil
	})
}

//

func RootAction(ctx *cli.Context) error {
	return cli.ShowAppHelp(ctx)
}

//

func NewApp() *cli.App {
	app := &cli.App{}

	app.Before = Before
	app.Flags = Flags
	app.Action = RootAction
	app.Commands = Commands
	app.Version = meta.Version

	return app
}

func Run() {
	err := NewApp().Run(os.Args)
	if err != nil {
		errors.Fatal(errors.Wrap(
			err, fmt.Sprintf(
				"pid: %d, ppid: %d",
				os.Getpid(), os.Getppid(),
			),
		))
	}
}
