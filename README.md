# Launcher2

Build and run discourse images.

## Changes from launcher

No prereqs are check yet. It assumes you have docker set up and whatever minimum requirements setup for Discourse: namely a recent enough version of docker, git.

## Migration from launcher

Some things are not yet implemented from launcher1.

`DOCKER_HOST_IP` - container can use `host.docker.internal`
stable `mac-address` - not implemented

No debug. Under the hood, launcher2 uses docker build which does not allow images to be saved along the way.

## New features

* Individual build, configure, and migrate commands. Adds the ability to partially build up an image "offline" without taking down an image until what is absolutely necessary, saving downtime.
* Adds the ability to build and run an image that finishes a build on boot, allowing the server to display a build page.
* Multiline env support
* docker compose generation. Allows exporting of configuration from discourse's pups configuration to a docker compose configuration.
