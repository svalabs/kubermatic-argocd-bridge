FROM golang as builder

WORKDIR /app

COPY . /app/

RUN go mod download

WORKDIR /app/cmd/
RUN CGO_ENABLED=0 go build -o kubermatic-argocd-bridge


FROM alpine
COPY --from=builder /app/cmd/kubermatic-argocd-bridge /usr/local/bin/kubermatic-argocd-bridge

CMD "/usr/local/bin/kubermatic-argocd-bridge"