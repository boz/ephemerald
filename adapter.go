package ephemerald

import (
	"context"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/config"
	"github.com/boz/ephemerald/params"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/nat"
)

type Adapter interface {
	Ref() reference.Named
	EnsureImage() error
	CreateContainer() (string, error)
	ContainerStart(id string, options types.ContainerStartOptions) error

	ContainerInspect(id string) (types.ContainerJSON, error)
	ContainerKill(id string, signal string) error
	Events(options types.EventsOptions) (<-chan events.Message, <-chan error)
	ContainerLogs(id string, options types.ContainerLogsOptions) (io.ReadCloser, error)

	MakeParams(StatusItem) (params.Params, error)

	Log() logrus.FieldLogger
}

type adapter struct {
	config *config.Config

	// docker image reference
	ref  reference.Named
	info *registry.RepositoryInfo

	// docker client
	client *client.Client

	ctx context.Context

	log logrus.FieldLogger
}

func newAdapter(config *config.Config) (Adapter, error) {

	log := config.Log().WithField("component", "adapter")

	ref, err := reference.ParseNormalizedNamed(config.Image)

	if err != nil {
		log.WithError(err).
			Errorf("Unable to parse image '%s'", config.Image)
		return nil, err
	}

	log = log.WithField("image", ref.String())

	info, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		log.WithError(err).Error("unable to parse repository info")
		return nil, err
	}

	client, err := client.NewEnvClient()
	if err != nil {
		log.WithError(err).Error("unable to crate docker client")
		return nil, err
	}

	return &adapter{
		config: config,
		ref:    ref,
		info:   info,
		client: client,
		log:    log,
		ctx:    context.Background(),
	}, nil
}

func (a *adapter) Ref() reference.Named {
	return a.ref
}

func (a *adapter) EnsureImage() error {
	exists, err := a.ImageExists()
	if err != nil {
		return err
	}

	if exists {
		a.log.Info("found image")
		return nil
	}

	a.log.Warn("image not present")
	return a.PullImage()
}

func (a *adapter) ImageExists() (bool, error) {
	_, _, err := a.client.ImageInspectWithRaw(a.ctx, a.ref.Name())
	switch {
	case err == nil:
		return true, nil
	case client.IsErrImageNotFound(err):
		return false, nil
	default:
		a.log.WithError(err).
			Errorf("error inspecting image")
		return false, err
	}
}

func (a *adapter) PullImage() error {
	a.log.Infof("pulling image...")

	body, err := a.client.ImageCreate(a.ctx, a.ref.String(), types.ImageCreateOptions{})
	if err != nil {
		a.log.WithError(err).
			Error("error pulling image")
		return err
	}

	defer body.Close()

	_, err = io.Copy(ioutil.Discard, body)
	if err != nil {
		a.log.WithError(err).
			Error("error while pulling image")
		return err
	}

	a.log.Info("done pulling image")
	return nil
}

func (a *adapter) CreateContainer() (string, error) {

	dconfig := &container.Config{
		Image:        a.ref.Name(),
		Cmd:          a.config.Container.Cmd,
		Env:          a.config.Container.Env,
		Volumes:      a.config.Container.Volumes,
		Labels:       a.config.Container.Labels,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		ExposedPorts: nat.PortSet{
			nat.Port(strconv.Itoa(a.config.Port)): struct{}{},
		},
	}

	hconfig := &container.HostConfig{
		AutoRemove:      true,
		PublishAllPorts: true,
		RestartPolicy:   container.RestartPolicy{},
	}

	nconfig := &network.NetworkingConfig{}

	name := ""

	container, err := a.client.ContainerCreate(a.ctx, dconfig, hconfig, nconfig, name)
	if err != nil {
		a.log.WithError(err).Error("can't create container")
		return "", err
	}

	lcid(a.log, container.ID).Infof("container created")
	for _, w := range container.Warnings {
		lcid(a.log, container.ID).Warn(w)
	}

	return container.ID, nil
}

func (a *adapter) ContainerStart(id string, options types.ContainerStartOptions) error {
	return a.client.ContainerStart(a.ctx, id, options)
}

func (a *adapter) ContainerInspect(id string) (types.ContainerJSON, error) {
	return a.client.ContainerInspect(a.ctx, id)
}

func (a *adapter) ContainerKill(id string, signal string) error {
	return a.client.ContainerKill(a.ctx, id, signal)
}

func (a *adapter) Events(options types.EventsOptions) (<-chan events.Message, <-chan error) {
	return a.client.Events(a.ctx, options)
}

func (a *adapter) ContainerLogs(id string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return a.client.ContainerLogs(a.ctx, id, options)
}

func (a *adapter) MakeParams(c StatusItem) (params.Params, error) {
	return a.config.Params.ParamsFor(c.ID(), c.Status(), a.config.Port)
}

func (a *adapter) Log() logrus.FieldLogger {
	return a.log
}
