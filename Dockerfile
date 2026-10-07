# Image is built by GoReleaser. The binary is cross-compiled outside this
# Dockerfile (per target platform) and made available in the build context
# as `gomddoc`. Theme assets are embedded in the binary via //go:embed, so
# no extra files are needed at runtime.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

COPY gomddoc /gomddoc

EXPOSE 8080

ENTRYPOINT ["/gomddoc"]
