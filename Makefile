IMAGE ?= quay.io/ulrichschreiner/s3syncer
SHA ?= g$(shell git rev-parse --short=8 HEAD)

define TEST_RQ
{ "events":[ \
	{ \
		"id":"4e9307b3-84e1-4556-8405-60e441efbad4", \
		"timestamp":"2021-01-27T08:05:22.885090115Z", \
		"action":"push", \
		"target":{ \
			"mediaType":"application/vnd.docker.distribution.manifest.v2+json", \
			"size":1152, \
			"digest": \
			"sha256:134c7fe821b9d359490cd009ce7ca322453f4f2d018623f849e580a89a685e5d", \
			"length":1152, \
			"repository":"developer/test", \
			"url":"https://rg.local.minikube/v2/developer/test/manifests/sha256:134c7fe821b9d359490cd009ce7ca322453f4f2d018623f849e580a89a685e5d", \
			"tag":"latest" \
		}, \
		"request":{ \
			"id":"bdefdc89-93db-4146-bbb3-39af3e006bfc", \
			"addr":"192.168.49.1", \
			"host":"rg.local.minikube", \
			"method":"PUT", \
			"useragent":"docker/20.10.2 go/go1.15.6 git-commit/8891c58a43 kernel/5.10.7-arch1-1 os/linux arch/amd64 UpstreamClient(Docker-Client/20.10.2 \\(linux\\))" \
		}, \
		"source":{ \
			"addr":"gitlab-registry-76bbb9fcb7-xdq2j:5000", \
			"instanceID":"e6af7e2a-79fc-4544-8514-41f5e7612775" \
		} \
	} \
  ] \
}
endef

all:
	mkdir -p bin
	go build -o bin/s3syncer

.PHONY:
run:
	SYNC_LISTEN=:9999 \
	SYNC_COMMAND_TEST='a bc cddd' \
	bin/s3syncer -config test/localtest.yml

.PHONY:
testcall:
	curl -H "Content-Type: application/json" --data '$(TEST_RQ)' http://localhost:9999/trigger/test1

.PHONY:
dockerimage:
	docker build -t $(IMAGE):latest -t $(IMAGE):$(SHA) .