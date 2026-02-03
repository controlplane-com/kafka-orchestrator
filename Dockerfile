FROM golang:1.25 AS builder

ARG COMPONENT
ARG PROJECT_DIR

COPY ${PROJECT_DIR}/go.mod ${PROJECT_DIR}/go.sum /${COMPONENT}/
WORKDIR /${COMPONENT}
RUN go mod download

COPY ${PROJECT_DIR}/cmd /${COMPONENT}/cmd
COPY ${PROJECT_DIR}/pkg /${COMPONENT}/pkg
RUN go mod tidy
WORKDIR /${COMPONENT}

ARG PROJECT_BUILD
ARG PROJECT_EPOCH
ARG PROJECT_VERSION
ARG PROJECT_TIMESTAMP
ARG VPREFIX
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X ${VPREFIX}.Epoch=${PROJECT_EPOCH} -X ${VPREFIX}.Version=${PROJECT_VERSION} -X ${VPREFIX}.Timestamp=${PROJECT_TIMESTAMP} -X ${VPREFIX}.Build=${PROJECT_BUILD}" -trimpath -v -o /${COMPONENT}/${COMPONENT} ./cmd/sidecar

FROM golang:1.25 as tester

ARG UID
ARG GID
ARG COMPONENT

RUN addgroup --gid $GID nonroot && adduser --uid $UID --gid $GID --disabled-password --gecos "" nonroot
USER nonroot
WORKDIR /home/nonroot

COPY --chown=nonroot:nonroot ./${COMPONENT}/go.mod ./${COMPONENT}/go.sum /home/nonroot/service/
WORKDIR /home/nonroot/service
RUN go mod download
COPY --chown=nonroot:nonroot ./${COMPONENT}/pkg /home/nonroot/service/pkg
COPY --chown=nonroot:nonroot ./${COMPONENT}/cmd /home/nonroot/service/cmd
RUN go mod tidy

RUN chmod -R 755 /home/nonroot/service/pkg

FROM alpine:3.20 as runner
RUN apk --no-cache add ca-certificates

ARG PROJECT_DIR
ARG COMPONENT

RUN mkdir /service
COPY --from=builder /${COMPONENT} /service

WORKDIR /service
ENV COMPONENT ${COMPONENT}
CMD ./$COMPONENT
