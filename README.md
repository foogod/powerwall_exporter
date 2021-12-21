# Powerwall metrics exporter for Prometheus / OpenMetrics

This exporter can be used to pull metrics from a Tesla Powerwall (and its associated Tesla Energy Gateway) into Prometheus, (or other collection systems which can pull data in OpenMetrics format via HTTP).

This exporter uses the Tesla Powerwall local-network API to login to the device and pull information.  Note that this requires that your Powerwall gateway be set up to connect to your local network (e.g. via WiFi or Ethernet).  For information on configuring and accessing your Powerwall over the local network, see [Tesla's online documentation](https://www.tesla.com/support/energy/powerwall/own/monitoring-from-home-network).

Once you have your Powerwall connected to your local network, it is recommended that you login and change the customer password.  This will then be the password you use with the `powerwall_exporter` config (below) as well.

---

***Many thanks to [Vince Loschiavo](https://github.com/vloschiavo) and other contributors to https://github.com/vloschiavo/powerwall2 for providing a lot of the information to make this possible!***

**Note:** The Tesla powerwall interface is an undocumented API which is not supported by Tesla, and could change at any time.  Moreover, updates to the Powerwall software are downloaded automatically by the device and typically installed without warning, so it is theoretically possible that things will just break unexpectedly.  I will try my best to keep this exporter up to date with any changes, but can't make any guarantees.

---

## How to Use

### Install from pre-built binaries

Pre-built binaries for common platforms can be found on the [Github releases page](https://github.com/foogod/powerwall_exporter/releases).  To use them, simply download, unpack, and copy the `powerwall_exporter` binary to wherever you would like it to live.

### Compiling from source

This requires having a system with the Go language development environment installed (which is actually usally not that difficult).  Please see the docs on [Downloading and Installing Go](https://go.dev/doc/install) for how to do this for various platforms.  Note that you *do not* need to compile the binary on the same platform which you will be running it on (you can, for example, use a Mac or Windows machine to compile a binary which will run on Linux, etc).

Once you have Go installed on your system:

1. Clone this repository to your local system
2. Run `go build` to build the binary (note, if you want to compile for a different architecture, you will need to set your `GOOS` and `GOARCH` environment variables appropriately first (see [examples here](https://freshman.tech/snippets/go/cross-compile-go-programs/)), for example: `GOOS=linux GOARCH=amd64 go build`)
3. Take the resulting `powerwall_exporter` executable (produced in the current directory) and place it wherever you want it to live.

### Configuring and starting it up

You will need to create a configuration file (see the [Config File](#config-file) section below) and place it somewhere where the program can read it.  If you wish to perform TLS certificate validation (see the [TLS Certificates](#tls-certificates) section), once you have created the config file, you will then want to run the following command to generate the cert file:

```
powerwall_exporter --config.file=<filename> --fetchcert
```

Then you will most likely want to configure your operating system to start up the exporter automatically as a service.  The details for doing this vary from one OS to another, but if you are running Systemd, a [sample unit file](examples/powerwall_exporter.service) to run the exporter has been included in the the examples directory, which can be used as a starting point.

The exporter does not require any special permissions (except enough to read its own config file and open network connections to the gateway).  It is therefore recommended that it *not* be run as root, but instead as some unprivileged user, for improved security.

Note that the config file will contain your Powerwall password, so it should ideally be readable only by the user that `powerwall_exporter` is running as.

### Pulling the metrics into Prometheus

Once the `powerwall_exporter` is up and running, you will need to configure Prometheus to pull metrics from it.  You can do this by adding something similar to the following to the `scrape_configs` section of the Prometheus config file (this assumes that the exporter is running on the same host as Prometheus):

```yaml
  - job_name: "powerwall"
    scrape_interval: 1m
    scrape_timeout: 1m
    static_configs:
      - targets:
          - "localhost:9871"

```

(Obviously, you can set the `scrape_interval` and `scrape_timeout` however you wish (or just leave them out to use the global defaults), however if they are set to scrape too frequently, you may experience occasional gaps in your data due to Powerwall connection issues.  See the next section for details and mitigations.)

## Gaps in data and retrying connections

The Tesla Energy Gateway devices seem to be remarkably bad at maintaining a reliable connection to WiFi networks (at least in many cases), and appear to just sort of "fall off" the network periodically for a minute or so before reconnecting.  This can cause problems if Prometheus attempts to scrape the data at that moment, and will result in gaps in the data for those points in time.

To mitigate this, the `powerwall_exporter` does support retrying connection attempts if it is unable to connect to the gateway, before giving up.  This is controlled by the `retry_interval` and `retry_timeout` settings in the config file (if the exporter is unable to connect to the gateway, it will keep trying every `retry_interval`, until `retry_timeout` has passed, at which point it will then finally give up and return without data)

Note that there is no reason to set the exporter's `retry_timeout` any larger than the `scrape_timeout` configured in Prometheus, because even if the exporter does eventually come back with some results, Prometheus will just ignore them anyway.  Moreover, Prometheus will not allow setting the `scrape_timeout` longer than the `scrape_interval`, which means that the scrape interval determines an upper bound on how long the exporter can reasonably wait for a response from the gateway.  Because of this, if you, for example, set the Prometheus `scrape_interval` to pull metrics every 15 seconds, then when the gateway disconnects from the network for a minute or so, there will be two or three sample intervals with no data, and there is just no way to get around that.

In general, it is recommended to set your `scrape_interval` and `scrape_timeout` to at least 1 or 2 minutes (or more), and set the exporter's `retry_timeout` to the same value, if you want to avoid gaps in your data when accessing the powerwall gateway over a WiFi network.

## TLS certificates

The Powerwall device communicates via HTTPS, but it uses a self-signed certificate, which means that normal certificate validation will fail (because it is not signed by a trusted authority).  For this reason, by default, `powerwall_exporter` does not attempt to validate the TLS certificate it receives when connecting.  This works, but it is insecure.

The exporter can validate the TLS certificate, but it requires first downloading and storing a copy of it in a file.  This can be done by setting the `tls_cert_file` setting in the config file to point to the location of a PEM-encoded certificate file.

Once you have configured the `gateway_address` and the `tls_cert_file` in the config file, you can actually tell `powerwall_exporter` to download the cert and generate the file for you automatically, like so:

```
powerwall_exporter --config.file=<filename> --fetchcert
```

This will read the filename from the config file, and write the fetched certificate to that file (creating it if it does not already exist).  Note that you should only really ever have to do this once (when you first set up the exporter), as the certificate should not change from then on (if it does, something suspicious may be going on).

## Command-line options

The `powerwall_exporter` supports the following command-line options:

- `--debug` -- Enable debugging output
- `--config.file=<filename>` -- Specify the location of the config file
- `--log.style=<option>` -- Specify the style of log output desired.  Valid options are `text`, `logfmt`, or `json` (default is `logfmt`).
- `--fetchcert` -- Instead of normal operation, connect to the powerwall and download its TLS certificate, and save it in the `tls_cert_file` specified in the configuration

## Config file

By default, the `powerwall_exporter` will search for a config file in the same directory as the program, with a name of `powerwall_exporter.yaml` to read its configuration.  You can change this location by passing the `--config.file=<path>` option on the command line.  The configuration file is a YAML file with the following supported options:

### `web` section

This section contains general parameters about how the exporter listens for and serves incoming connections.  Possible parameters are:

- `listen_address` -- The IP address and port to listen for HTTP connections (defaults to ":9871")
- `metrics_path` -- The HTTP path to serve metrics on (defaults to "/metrics")

### `device` section

This section contains information about the Tesla device to connect and pull metrics from.  Possible parameters are:

- `gateway_address` -- The IP address or hostname of the Tesla Energy Gateway to connect to
- `login_email` -- The email address to use when logging into the gateway (customer login email)
- `login_password` -- The password to use when logging into the gateway (customer login password)
- `tls_cert_file` -- PEM file containing the gateway's TLS certificate (for validation)
- `retry_interval` -- How long to wait between retries on connection failure
- `retry_timeout` -- How long to retry connections before giving up

Note that `gateway_address` and `login_password` are required parameters.  All others are optional.

`retry_interval` and `retry_timeout` are expressed in duration-string notation; for example, `37s`, `1m20s`, `5d12h10.07s`, etc.  (though that last one is probably a bit long of a duration for this sort of thing..)

### Example config file

The following is a sample YAML config file for reference (note that most of these parameters are optional):

```yaml
web:
  listen_address: "0.0.0.0:9871"
  metrics_path: "/metrics"
device:
  gateway_address: "powerwall"
  login_email: "powerwalluser@example.com"
  login_password: "Super!Secret!Password"
  tls_cert_file: "powerwall_cert.pem"
  retry_interval: "1s"
  retry_timeout: "60s"
```

## Metrics and units

This exporter attempts to follow Prometheus best practices for metric names and units.  Because of this, some metrics are exported with slightly different names or units than presented via the Tesla API.

Metric values:

- Time values are reported in Unix seconds-since-epoch format
- Durations are reported in seconds
- Power is reported in Watts
- Many measurements of total energy are reported by the Powerwall in kilowatt-hours (kWh).  These are converted to Joules when exporting to Prometheus metrics
- Some values are reported by the Powerwall in percent.  These are converted to ratios (0.0-1.0) for the Prometheus metrics.

Metric naming:

- All metric names are prefixed with `powerwall_`
- Metrics which are specific to a particular sub-device (i.e. a particular battery pack) will have an additional prefix of `dev_` (after `powerwall_`), with a `serial=` label to indicate the serial number of the specific device they apply to.
- Names of non-boolean metrics have a suffix indicating the unit (`_seconds`, `_watts`, etc)
- Additionally, if a metric represents a (always-increasing) counter, it has a suffix of `_total` to indicate this (for these sorts of metrics you will usually want to take the rate of change over time, instead of looking at the raw number)
- States or modes which can be in one of several conditions are represented by a metric which always has a value of 1, with a label (such as `state=` or `mode=`) which indicates which state is being reported.

The `powerwall_info` metric also reports (via its labels) some general non-numeric information about the Powerwall, such as current version of the software it is running, etc (it always has a value of 1 if present).  (The presence or absence of this metric can also be used to determine whether or not the exporter was able to communicate with the Powerwall at all.)
