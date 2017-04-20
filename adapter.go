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

type dockerAdapter interface {
	imageReference() reference.Named
	ensureImage() error
	createContainer() (string, error)
	containerStart(id string, options types.ContainerStartOptions) error

	containerInspect(id string) (types.ContainerJSON, error)
	containerKill(id string, signal string) error
	containerEvents(options types.EventsOptions) (<-chan events.Message, <-chan error)
	containerLogs(id string, options types.ContainerLogsOptions) (io.ReadCloser, error)

	makeParams(StatusItem) (params.Params, error)

	logger() logrus.FieldLogger
}

type dadapter struct {
	config *config.Config

	// docker image reference
	ref  reference.Named
	info *registry.RepositoryInfo

	// docker client
	client *client.Client

	ctx context.Context

	log logrus.FieldLogger
}

func newDockerAdapter(config *config.Config) (dockerAdapter, error) {

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

	return &dadapter{
		config: config,
		ref:    ref,
		info:   info,
		client: client,
		log:    log,
		ctx:    context.Background(),
	}, nil
}

func (a *dadapter) imageReference() reference.Named {
	return a.ref
}

func (a *dadapter) ensureImage() error {
	exists, err := a.imageExists()
	if err != nil {
		return err
	}

	if exists {
		a.log.Info("found image")
		return nil
	}

	a.log.Warn("image not present")
	return a.imagePull()
}

func (a *dadapter) imageExists() (bool, error) {
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

func (a *dadapter) imagePull() error {
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

func (a *dadapter) createContainer() (string, error) {

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

func (a *dadapter) containerStart(id string, options types.ContainerStartOptions) error {
	return a.client.ContainerStart(a.ctx, id, options)
}

func (a *dadapter) containerInspect(id string) (types.ContainerJSON, error) {
	return a.client.ContainerInspect(a.ctx, id)
}

func (a *dadapter) containerKill(id string, signal string) error {
	return a.client.ContainerKill(a.ctx, id, signal)
}

func (a *dadapter) containerEvents(options types.EventsOptions) (<-chan events.Message, <-chan error) {
	return a.client.Events(a.ctx, options)
}

func (a *dadapter) containerLogs(id string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return a.client.ContainerLogs(a.ctx, id, options)
}

func (a *dadapter) makeParams(c StatusItem) (params.Params, error) {
	return a.config.Params.ParamsFor(c.ID(), c.Status(), a.config.Port)
}

func (a *dadapter) logger() logrus.FieldLogger {
	return a.log
}
