FROM golang as builder

WORKDIR /app

COPY . /app/

RUN go mod download

RUN cd cmd

RUN CGO_ENABLED=0 go build -o kubermatic-argocd-bridge

CMD "./kubermatic-argocd-bridge"