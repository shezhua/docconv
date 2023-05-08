FROM golang:1.20 AS build

WORKDIR /app
COPY . ./
RUN apt-get update && apt-get install -y libtesseract-dev=4.1.1-2.1 tesseract-ocr=4.1.1-2.1
RUN go build -tags ocr -o /bin/docd ./docd

################################################################################

FROM debian:11-slim AS docd

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    tesseract-ocr=4.1.1-2.1\
    tesseract-ocr-chi-sim \
    lynx \
    poppler-utils \
    tidy \
    unrtf \
    wv \
    zip \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN update-ca-certificates

EXPOSE 8888

COPY --from=build /bin/docd /docd
ENTRYPOINT ["/docd"]
CMD ["--help"]
