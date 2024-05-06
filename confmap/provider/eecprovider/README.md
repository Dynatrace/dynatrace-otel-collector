# EEC Provider

This is an OpenTelemetry Collector `confmap.Provider` module that allows the
Collector to be configured with the Dynatrace Extensions Execution Controller.

> [!WARNING]
> This is an internal component not intended for direct customer use, but is only intended for use by
> Collectors installed and managed by Dynatrace. Configuring this
> component directly is not supported.

## Configuration

This confmap Provider can be configured by pasing query parameter-formatted values
inside the fragment of the URL given to the config flag. For example:

```shell
dynatrace-otel-collector --config=eec://my.eec.host:31098#refresh-interval=5s&auth-file=/var/private/token.key
```

### Options

| Key | Default | Description |
|-----|---------|-------------|
| auth-env |  None | An environment variable that will be read to get a plaintext API token or other key to be used in an HTTP header. Mutually exclusive from `auth-file`, passing both options will result in an error. |
| auth-file | None | A filepath containing a plaintext version of an API token or other key to be used in an HTTP header to authenticate with the EEC. |
| refresh-interval | 10s | A time duration that defines how frequently the provider should check the given URL for updates. |
| timeout | 8s | A time duration that defines how long the provider will wait until cancelling an ongoing HTTP request. |
| insecure | false | If set to "true", use HTTP for the connection to the server. If unset or set to "false", use HTTPS. |
