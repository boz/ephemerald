package cpool

import (
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

var (
	utilLog = logrus.StandardLogger().
		WithField("package", "github.com/ovrclk/cpool").
		WithField("module", "util")
)

func createContainer(p *pool, ref reference.Named, config *Config) (string, error) {
	log := utilLog.WithField("image", ref.String())

	dconfig := &container.Config{
		Image:        ref.Name(),
		Cmd:          config.Cmd,
		Env:          config.Env,
		Volumes:      config.Volumes,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		ExposedPorts: make(nat.PortSet),
	}

	for p := range config.Ports {
		dconfig.ExposedPorts[p] = struct{}{}
	}

	hconfig := &container.HostConfig{
		AutoRemove:      true,
		PublishAllPorts: true,
		RestartPolicy:   container.RestartPolicy{},
	}

	nconfig := &network.NetworkingConfig{}

	name := ""

	container, err := p.client.ContainerCreate(p.ctx, dconfig, hconfig, nconfig, name)
	if err != nil {
		log.WithError(err).Error("can't create container")
		return "", err
	}

	utilLog.Infof("Created container %v", container.ID)
	for _, w := range container.Warnings {
		log.Warn(w)
	}

	return container.ID, nil
}

func ensureImageExists(p *pool, ref reference.Named) error {
	log := utilLog.WithField("image", ref.String())

	exists, err := imageExists(p, ref)
	if err != nil {
		return err
	}

	if exists {
		log.WithField("image", ref.Name()).Infof("found image")
		return nil
	}

	log.WithField("image", ref.Name()).
		Infof("image not present")

	return pullImage(p, ref)
}

func imageExists(p *pool, ref reference.Named) (bool, error) {
	log := utilLog.WithField("image", ref.String())

	_, _, err := p.client.ImageInspectWithRaw(p.ctx, p.ref.Name())
	switch {
	case err == nil:
		return true, nil
	case client.IsErrImageNotFound(err):
		return false, nil
	default:
		log.WithError(err).
			WithField("image", ref.Name()).
			Errorf("error inspecting image")
		return false, err
	}
}

func pullImage(p *pool, ref reference.Named) error {
	log := utilLog.WithField("image", ref.String())

	log.Infof("pulling image...")

	body, err := p.client.ImageCreate(p.ctx, ref.String(), types.ImageCreateOptions{})
	if err != nil {
		log.WithError(err).
			Error("error pulling image")
		return err
	}

	defer body.Close()

	buf := make([]byte, 1024)

	for {
		_, err := body.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.WithError(err).
				Error("error while pulling image")
			return err
		}
	}

	log.Info("done pulling image")
	return nil
}

func TCPPorts(status types.ContainerJSON) map[string]string {
	ports := make(map[string]string)

	if status.Config == nil {
		return ports
	}
	if status.NetworkSettings == nil {
		return ports
	}

	for port, _ := range status.Config.ExposedPorts {
		if port.Port() != "tcp" {
			continue
		}
		eport, ok := status.NetworkSettings.Ports[port]
		if !ok || len(eport) == 0 {
			continue
		}
		ports[port.Port()] = eport[0].HostPort
	}

	return ports
}
