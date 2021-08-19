# Setting up Monitoring Dashboard

## Motivation
GoShimmer is shipped with its internal node dashboard that you can reach at `127.0.0.1:8081` by default. While this dashboard provides some basic metrics information, its main functionality is to provide a graphical interface to interact with your node.

Node operators who wish to have more insights into what is happening within their node have the option to enable a [Prometheus](https://prometheus.io/) exporter plugin that gathers important metrics about their node. To visualize these metrics, a [Grafana Dashboard](https://grafana.com/oss/grafana/) is utilized.

# Setting up (run GoShimmer from a VPS)
To enable the **Monitoring Dashboard** for a GoShimmer node running from a VPS as described [here](setup.md), you need to carry out some additional steps.
1. Edit `docker-compose.yml`
   TODO
2. Create Prometheus config.
   TODO
3. Create Grafana config.
   TODO
4. Run `docker-compose up`.
   TODO

# Setting up (run GoShimmer from your home machine)
Depending on how you run your GoShimmer node, there are different ways to set up the **Monitoring Dashboard**.
## Docker
One of the easiest ways to run a node is to use [Docker](https://www.docker.com/). To automatically launch GoShimmer and the Monitoring Dashboard with docker, follow these steps:
1. [Install docker](https://docs.docker.com/get-docker/). On Linux, make sure you install both the [Docker Engine](https://docs.docker.com/engine/install/) and [Docker Compose](https://docs.docker.com/compose/install/).
2. Clone the GoShimmer repository.
   ```bash
   $ git clone git@github.com:iotaledger/goshimmer.git
   ```
3. Create a `config.json` from the provided `config.default.json`.
   ```bash
   $ cd goshimmer
   $ cp config.default.json config.json
   ```
   Make sure, that following entry is present in `config.json`:
   ```json
   {
     "prometheus": {
       "bindAddress": "127.0.0.1:9311"
     }
   }
   ```
4. From the root of the repo, start GoShimmer with:
   ```bash
   $ docker-compose up
   ```

You should be able to reach the Monitoring Dashboard via browser at [localhost:3000](http://localhost:3000). Default login credentials are:
* `username` : admin
* `password` : admin

After initial login, you will be prompted to change your password.
You can experiment with the dashboard, change layout, add panels and discover metrics. Your changes will be saved into a Grafana database located in the repo at `tools/monitoring/grafana/grafana.db`.

## Binary
If you run the [released binaries](https://github.com/iotaledger/goshimmer/releases), or build GoShimmer from source, you need to setup Prometheus and Grafana separately, furthermore, you have to configure GoShimmer to export data.

### GoShimmer Configuration
1. Make sure that the `prometheus.bindAddress` config parameter is set in your `config.json`:
   ```json
     {
        "prometheus": {
          "bindAddress": "127.0.0.1:9311"
        }
      }
   ```
2. Make sure, that the `prometheus` plugin is enabled in your `config.json`:
   ```json
   {
     "node": {
       "disablePlugins": [
         
       ],
       "enablePlugins": [
         "prometheus"
       ]
     }
   }
   ```
### Install and Configure Prometheus
First, we take a look on how to configure and run Prometheus as a standalone application. Then, we setup a Linux system service that automatically runs Prometheus in the background.
#### Prometheus as Standalone App
1. [Download](https://prometheus.io/download/) the latest release of Prometheus for your system.
2. Unpack the downloaded file:
   ```bash
   $ tar xvfz prometheus-*.tar.gz
   $ cd prometheus-*
   ```
3. Create a `prometheus.yml` in the unpacked directory with the following content:
   ```yaml
   scrape_configs:
       - job_name: goshimmer_local
         scrape_interval: 5s
         static_configs:
         - targets:
           # goshimmer prometheus plugin export
           - 127.0.0.1:9311
   ```
4. Start Prometheus from the unpacked folder:
   ```bash
   # By default, Prometheus stores its database in ./data (flag --storage.tsdb.path).
   $ ./prometheus --config.file=prometheus.yml
   ```
5. You can access the prometheus server at [localhost:9090](http://localhost:9090).
6. (Optional) Prometheus server is running, but observe that [localhost:9090/targets](http://localhost:9090/targets) shows the target being `DOWN`. Run GoShimmer with the configuration from the previous stage, and you will soon see the `goshimmer_local` target being `UP`.

#### Prometheus as a System Service (Linux)
Note: you have to have root privileges with your user to carry out the following steps.
1. Create a Prometheus user, directories, and set this user as the owner of those directories.
   ```
   $ sudo useradd --no-create-home --shell /bin/false prometheus
   $ sudo mkdir /etc/prometheus
   $ sudo mkdir /var/lib/prometheus
   $ sudo chown prometheus:prometheus /etc/prometheus
   $ sudo chown prometheus:prometheus /var/lib/prometheus
   ```
2. Download Prometheus source, extract and rename.
   ```
   $ wget https://github.com/prometheus/prometheus/releases/download/v2.19.1/prometheus-2.19.1.linux-amd64.tar.gz
   $ tar xvfz prometheus-2.19.1.linux-amd64.tar.gz
   $ mv prometheus-2.19.1.linux-amd64.tar.gz prometheus-files
   ```
3. Copy Prometheus binaries to `/bin` and change their ownership
   ```
   $ sudo cp prometheus-files/prometheus /usr/local/bin/
   $ sudo cp prometheus-files/promtool /usr/local/bin/
   $ sudo chown prometheus:prometheus /usr/local/bin/prometheus
   $ sudo chown prometheus:prometheus /usr/local/bin/promtool
   ```
4. Copy Prometheus console libraries to `/etc` and change their ownership.
   ```
   $ sudo cp -r prometheus-files/consoles /etc/prometheus
   $ sudo cp -r prometheus-files/console_libraries /etc/prometheus
   $ sudo chown -R prometheus:prometheus /etc/prometheus/consoles
   $ sudo chown -R prometheus:prometheus /etc/prometheus/console_libraries
   ```
5. Create Prometheus config file, define targets.
   To create and open up the config file:
   ```
   $ sudo nano /etc/prometheus/prometheus.yml
   ```
   Put the following content into the file:
   ```yaml
   scrape_configs:
       - job_name: goshimmer_local
         scrape_interval: 5s
         static_configs:
         - targets:
           # goshimmer prometheus plugin export
           - 127.0.0.1:9311
   ```
   Save and exit the editor.
6. Change ownership of the config file.
   ```
   $ sudo chown prometheus:prometheus /etc/prometheus/prometheus.yml
   ```
7. Create a Prometheus service file.
   ```
   $ sudo nano /etc/systemd/system/prometheus.service
   ```
   Copy the following content into the file:
   ```yaml
   [Unit]
   Description=Prometheus GoShimmer Server
   Wants=network-online.target
   After=network-online.target
   
   [Service]
   User=prometheus
   Group=prometheus
   Type=simple
   ExecStart=/usr/local/bin/prometheus \
       --config.file /etc/prometheus/prometheus.yml \
       --storage.tsdb.path /var/lib/prometheus/ \
       --web.console.templates=/etc/prometheus/consoles \
       --web.console.libraries=/etc/prometheus/console_libraries
   
   [Install]
   WantedBy=multi-user.target
   ```
8. Reload `systemd` service to register the prometheus service.
   ```
   $ sudo systemctl daemon-reload
   $ sudo systemctl start prometheus
   ```
9. Check if the service is running.
   ```
   $ sudo systemctl status prometheus
   ```
10. You can access the prometheus server at [localhost:9090](http://localhost:9090).
11. (Optional) Prometheus server is running, but observe that [localhost:9090/targets](http://localhost:9090/targets) shows the target being `DOWN`. Run GoShimmer with the configuration from the previous stage, and you will soon see the `goshimmer_local` target being `UP`.

+1. When you want to stop the service, run:
   ```
   $ sudo systemctl stop prometheus
   ```

Prometheus now collects metrics from your node, but we need to setup Grafana to visualize the collected data.

### Install and configure Grafana
Head over to [Grafana Documentation](https://grafana.com/docs/grafana/latest/installation/) and install Grafana. For Linux, the OSS Release is recommended.

#### Grafana as Standalaon App
Depending on where you install Grafana from, the configuration directories will change. For clarity, we will proceed with the binary install here.
1. [Download Grafana](https://grafana.com/grafana/download) binary and extract it into a folder.
   For example:
   ```bash
   $ wget https://dl.grafana.com/oss/release/grafana-7.0.4.linux-amd64.tar.gz
   $ tar -zxvf grafana-7.0.4.linux-amd64.tar.gz
   ```
2. We will need couple files from the GoShimmer repository. Here we suppose, that you have the repository directory `goshimmer` on the same level as the extracted `grafana-7.0.4` directory:
   ```
   ├── grafana-7.0.4   
   │   ├── bin       
   │   ├── conf         
   │   ├── LICENSE   
   │   ├── NOTICE.md
   │   ├── plugins-bundled
   │   ├── public 
   │   ├── README.md
   │   ├── scripts 
   │   └── VERSIO
   ├── goshimmer               
   │   ├── CHANGELOG.md
   │   ├── client             
   │   ├── config.default.json
       ...
   ```
   We copy a couple configuration files from the repository into Grafana's directory:
   ```
   $ cp -R goshimmer/tools/monitoring/grafana/dashboards/local_dashboard.json grafana-7.0.4/public/dashboards/
   $ cp goshimmer/tools/monitoring/grafana/provisioning/datasources/datasources.yaml grafana-7.0.4/conf/provisioning/datasources/datasources.yaml
   $ cp goshimmer/tools/monitoring/grafana/provisioning/dashboards/dashboards.yaml grafana-7.0.4/conf/provisioning/dashboards/dashboards.yaml
   ```

3. Run Grafana.
   ```
   $ cd grafana-7.0.4/bin
   $ ./grafana-server
   ```
4. Open Moitoring Dashboard at [localhost:3000](http://localhost:3000).

Default login credentials are:
* `username` : admin
* `password` : admin

#### Grafana as a System Service (Linux)
Instead of running the `grafana-server` app each time we can create a service that runs in the background.

When you install Grafana from
* [APT repository](https://grafana.com/docs/grafana/latest/installation/debian/#install-from-apt-repository) or `.deb` [package](https://grafana.com/docs/grafana/latest/installation/debian/#install-deb-package) (Ubuntu or Debian),
* [YUM repository](https://grafana.com/docs/grafana/latest/installation/rpm/#install-from-yum-repository) or `.rpm` [package](https://grafana.com/docs/grafana/latest/installation/rpm/#install-with-rpm) (CentOS, Fedora, OpenSuse, RedHat),

then Grafana is configured to run as a system service without any modification. All you need to do is copy config files from the GoShimmer repository:
1. Copy [datasource yaml config](https://github.com/iotaledger/goshimmer/blob/develop/tools/monitoring/grafana/provisioning/datasources/datasources.yaml) to `/etc/grafana`:
   (assuming you are at the root of the cloned GoShimmer repository)
   ```
   $ sudo cp tools/monitoring/grafana/provisioning/datasources/datasources.yaml /etc/grafana/provisioning/datasources
   ```
2. Copy [dashboard yaml config](https://github.com/iotaledger/goshimmer/blob/develop/tools/monitoring/grafana/provisioning/dashboards/dashboards.yaml) to `/etc/grafana`:
   ```
   $ sudo cp tools/monitoring/grafana/provisioning/dashboards/dashboards.yaml /etc/grafana/provisioning/dashboards
   ```
3. Copy [GoShimmer Local Metrics](https://github.com/iotaledger/goshimmer/blob/develop/tools/monitoring/grafana/dashboards/local_dashboard.json) dashboard to `/var/lib/grafana/`:
   ```
   $ sudo cp -R tools/monitoring/grafana/dashboards /var/lib/grafana/
   ```
4. Reload daemon and start Grafana.
   ```
   $ sudo systemctl daemon-reload
   $ sudo systemctl start grafana-server
   ```
5. Open Moitoring Dashboard at [localhost:3000](http://localhost:3000).

Default login credentials are:
* `username` : admin
* `password` : admin

#### Grafana Config via GUI
If you successfully installed Grafana and would like to set it up using its graphical interface, here are the steps you need to take:
1. Run Grafana.
2. Open [localhost:3000](http://localhost:3000) in a browser window.
   Default login credentials are:
   * `username` : admin
   * `password` : admin
3. On the left side, open **Configuration -> Data Sources**. Click on **Add data source** and select **Prometheus** core plugin.
4. Fill the following fields:
   - `URL`: http://localhost:9090
   - `Scrape interval`: 5s
5. Click on **Save & Test**. If you have a running Prometheus server, everything should turn green. If the URL can't be reached, try changing the **Access** field to `Browser`.
6. On the left side panel, click on **Dashboards -> Manage**.
7. Click on **Import**. Paste the content of [local_dashboard.json](https://github.com/iotaledger/goshimmer/blob/develop/tools/monitoring/grafana/dashboards/local_dashboard.json) in the **Import via panel json**, or download the life and use the **Upload .json file** option.
8. Now you can open **GoShimmer Local Metrics** dashboard under **Dashboards**. Don't forget to start your node and run Prometheus!