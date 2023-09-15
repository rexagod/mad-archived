ARG GOARCH=amd64
ARG GOVER=1.21

FROM golang:${GOVER}-alpine AS builder
ENV GOARCH=${GOARCH}

WORKDIR /go/src/github.com/rexagod/mad/
COPY . .

RUN make build

FROM gcr.io/distroless/static:latest-${GOARCH}

COPY --from=builder /go/src/github.com/rexagod/mad/mad /

ENV HOME /home/nonroot

RUN addgroup --system nonroot \
    && adduser \
		--system \
		--disabled-password \
		--gecos "" \
		--home $(echo $HOME) \
		--ingroup nonroot \
		nonroot \
    && chown -R nonroot:nonroot $(echo $HOME)

USER nonroot:nonroot

ENTRYPOINT ["/mad"]
