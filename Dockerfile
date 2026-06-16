# Image is built by GoReleaser. The binary is cross-compiled outside this
# Dockerfile (per target platform) and made available in the build context
# as `gomddoc`. Theme assets are embedded in the binary via //go:embed, so
# no extra files are needed at runtime.
FROM gcr.io/distroless/static-debian12:nonroot

COPY gomddoc /gomddoc

EXPOSE 8080

ENTRYPOINT ["/gomddoc"]
