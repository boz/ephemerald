package cpool

import (
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func createContainer(p *pool, ref reference.Named, config *Config) (string, error) {
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
		p.log.WithError(err).Error("can't create container")
		return "", err
	}

	p.log.Infof("Created container %v", container.ID)
	for _, w := range container.Warnings {
		p.log.WithField("container", container.ID).Warn(w)
	}

	return container.ID, nil
}

func ensureImageExists(p *pool, ref reference.Named) error {
	exists, err := imageExists(p, ref)
	if err != nil {
		return err
	}

	if exists {
		p.log.Infof("found image")
		return nil
	}

	p.log.Warnf("image not present")

	return pullImage(p, ref)
}

func imageExists(p *pool, ref reference.Named) (bool, error) {
	_, _, err := p.client.ImageInspectWithRaw(p.ctx, p.ref.Name())
	switch {
	case err == nil:
		return true, nil
	case client.IsErrImageNotFound(err):
		return false, nil
	default:
		p.log.WithError(err).
			Errorf("error inspecting image")
		return false, err
	}
}

func pullImage(p *pool, ref reference.Named) error {

	p.log.Infof("pulling image...")

	body, err := p.client.ImageCreate(p.ctx, ref.String(), types.ImageCreateOptions{})
	if err != nil {
		p.log.WithError(err).
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
			p.log.WithError(err).
				Error("error while pulling image")
			return err
		}
	}

	p.log.Info("done pulling image")
	return nil
}

func TCPPortsFor(status types.ContainerJSON) map[string]string {
	ports := make(map[string]string)

	if status.Config == nil {
		return ports
	}
	if status.NetworkSettings == nil {
		return ports
	}

	for port, _ := range status.Config.ExposedPorts {
		if port.Proto() != "tcp" {
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
