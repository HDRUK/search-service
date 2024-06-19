## Search Service Tiltfile
##
## Branwen Snelling <branwen.snelling@hdruk.ac.uk>
##

load('ext://restart_process', 'docker_build_with_restart')

cfg = read_json('tiltconf.json')

docker_build_with_restart(
    ref='hdruk/' + cfg.get('name'),
    context='.',
    entrypoint=["./search-service"],
    live_update=[
        sync('.', '/app'),
        run('go mod download', trigger='./go.mod'),
        run('go build --ldflags="-s -w" -o search-service'),
    ]
)

k8s_yaml('chart/' + cfg.get('name') + '/deployment.yaml')
k8s_yaml('chart/' + cfg.get('name') + '/service.yaml')
k8s_resource(
    cfg.get('name'),
    port_forwards=8080,
    labels=["API"]
)