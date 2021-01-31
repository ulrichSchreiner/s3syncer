# s3syncer

[![Build Status](https://github.drone.protegear.io/api/badges/ulrichSchreiner/s3syncer/status.svg)](https://github.drone.protegear.io/ulrichSchreiner/s3syncer)

`s3syncer` is a webhook for a docker distribution registry which will sync the
used s3 storage and sync the contents to another bucket.

Well, in reality it does not sync the storage it simply executes a predefined
command which **can** sync. The example in the `manifest/k8s.yaml` file configures
a command which executes an `mc mirror ...` command, when the hook is executed. The
command is executed when a webhook happens which contains a non-empty tag. This
is important to know: The command **will not be executed for every layer**.

You can define multiple hooks by configuring a list of commands. Every command
has a name and this name can be used in the hook URL as the last path part. So
for example if you have the following command defined:

```yml
      - name: push
        disable: true
        delay: 3s
        reconcile: 60s
        runOnStart: true
        workdir: /tmp
        cmd: /usr/local/bin/mc
        args:
          - mirror
          - local-minio/registry
          - remote-minio/registry
```

You now can use the URL `http://<address>:<port>/triger/push`. The listen
address can be specified with the `-listen` configuration, the configuration
file is specified with `-config`, the default is `/etc/s3syncer/config.yaml`.

If you want to use specific commands in the webhook, you have to create your own
image and include your commands/executables to the image. Take the given
K8S manifest as an example for installation of the hook.

## Config Options
Beneath the name you can set different parameters:

 - `disable`<br>
   The command itself will be disabled and not be executed.
 - `delay`<br>
   The command will be executed after this given duration of silence. Use this
   value to prevent a mass of consecutive calls to the command if many events
   happen. The systme will wait for the given delay and then trigger the
   command
 - `reconcile`<br>
   To make sure your comannd will be executed regularly you can specify a
   reconcile duration; after this duration the command will be called without
   a event. If you want to prevent errors when you have network failures, you
   should regularly reconcile the system.
 - `runOnStart`<br>
   As the name suggests, starts the command, when the `s3syncer` itself starts.
 - `workdir`<br>
   Specifiy the working directory for the triggered command.
 - `cmd` / `args`<br>
   Specify the command and the arguments to execute.

## Distribution Configuration

To call your hook you have to add this hook to your registry config:
```yml
- name: push_event
  url: http://s3syncer:9999/trigger/push
  timeout: 1s
  threshold: 5
  backoff: 1s
```
For mor information on this read [the docker docs](https://docs.docker.com/registry/notifications/).