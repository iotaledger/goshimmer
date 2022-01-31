import { action, computed, observable, ObservableMap } from 'mobx';
import * as dateformat from 'dateformat';
import { connectWebSocket, registerHandler, unregisterHandler, WSMsgType } from "app/misc/WS";

class MPSMetric {
    mps: number;
    ts: string;
}

class Status {
    id: string;
    version: string;
    uptime: number;
    mem: MemoryMetrics = new MemoryMetrics();
    tangleTime: TangleTime;
}

class TangleTime {
    synced: boolean;
    time: number;
    messageID: string;
}

class MemoryMetrics {
    heap_sys: number;
    heap_alloc: number;
    heap_idle: number;
    heap_released: number;
    heap_objects: number;
    last_pause_gc: number;
    num_gc: number;
    ts: string;
}

class TipsMetric {
    totaltips: number;
    ts: string;
}

class NetworkIO {
    tx: number;
    rx: number;
    ts: string;
}

class NeighborMetrics {
    @observable collected: Array<NeighborMetric> = [];
    @observable network_io: Array<NetworkIO> = [];

    addMetric(metric: NeighborMetric) {
        metric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.collected.push(metric);
        if (this.collected.length > maxMetricsDataPoints) {
            this.collected.shift();
        }
        let netIO = this.currentNetIO;
        if (netIO) {
            if (this.network_io.length > maxMetricsDataPoints) {
                this.network_io.shift();
            }
            this.network_io.push(netIO);
        }
    }

    get current() {
        return this.collected[this.collected.length - 1];
    }

    get secondLast() {
        let index = this.collected.length - 2;
        if (index < 0) {
            return
        }
        return this.collected[index];
    }

    get currentNetIO(): NetworkIO {
        if (this.current && this.secondLast) {
            return {
                tx: this.current.packets_written - this.secondLast.packets_written,
                rx: this.current.packets_read - this.secondLast.packets_read,
                ts: dateformat(new Date(), "HH:MM:ss"),
            };
        }
        return null;
    }

    @computed
    get netIOSeries() {
        let tx = Object.assign({}, chartSeriesOpts,
            series("Tx", 'rgba(53, 180, 219,1)', 'rgba(53, 180, 219,0.4)')
        );
        let rx = Object.assign({}, chartSeriesOpts,
            series("Rx", 'rgba(235, 134, 52)', 'rgba(235, 134, 52,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.network_io.length; i++) {
            let metric: NetworkIO = this.network_io[i];
            labels.push(metric.ts);
            tx.data.push(metric.tx);
            rx.data.push(-metric.rx);
        }

        return {
            labels: labels,
            datasets: [tx, rx],
        };
    }
}

class NeighborMetric {
    id: string;
    address: string;
    connection_origin: number;
    packets_read: number;
    packets_written: number;
    ts: number;
}

class ComponentCounterMetric {
    store: number;
    solidifier: number;
    scheduler: number;
    booker: number;
    ts: number;
}

const chartSeriesOpts = {
    label: "Incoming", data: [],
    fill: true,
    lineTension: 0,
    backgroundColor: 'rgba(58, 60, 171,0.4)',
    borderWidth: 1,
    borderColor: 'rgba(58, 60, 171,1)',
    borderCapStyle: 'butt',
    borderDash: [],
    borderDashOffset: 0.0,
    borderJoinStyle: 'miter',
    pointBorderColor: 'rgba(58, 60, 171,1)',
    pointBackgroundColor: '#fff',
    pointBorderWidth: 1,
    pointHoverBackgroundColor: 'rgba(58, 60, 171,1)',
    pointHoverBorderColor: 'rgba(220,220,220,1)',
    pointHoverBorderWidth: 2,
    pointRadius: 0,
    pointHitRadius: 20,
    pointHoverRadius: 5,
};

function series(name: string, color: string, bgColor: string) {
    return {
        label: name, data: [],
        backgroundColor: bgColor,
        borderColor: color,
        pointBorderColor: color,
        pointHoverBackgroundColor: color,
        pointHoverBorderColor: 'rgba(220,220,220,1)',
    }
}

const statusWebSocketPath = "/ws";

const maxMetricsDataPoints = 900;

export class NodeStore {
    @observable status: Status = new Status();
    @observable websocketConnected: boolean = false;
    @observable last_mps_metric: MPSMetric = new MPSMetric();
    @observable collected_mps_metrics: Array<MPSMetric> = [];
    @observable collected_mem_metrics: Array<MemoryMetrics> = [];
    @observable neighbor_metrics = new ObservableMap<string, NeighborMetrics>();
    @observable last_tips_metric: TipsMetric = new TipsMetric();
    @observable collected_tips_metrics: Array<TipsMetric> = [];
    @observable last_component_counter_metric: ComponentCounterMetric = new ComponentCounterMetric();
    @observable collected_component_counter_metrics: Array<ComponentCounterMetric> = [];
    @observable collecting: boolean = true;

    constructor() {
        this.status.tangleTime = new TangleTime;
        this.status.tangleTime.time = 0;
        this.registerHandlers();
    }

    registerHandlers = () => {
        registerHandler(WSMsgType.Status, this.updateStatus);
        registerHandler(WSMsgType.MPSMetrics, (mps: number) => {
            this.addMPSMetric(this.updateLastMPSMetric(mps));
        });
        registerHandler(WSMsgType.NeighborStats, this.updateNeighborMetrics);
        registerHandler(WSMsgType.TipsMetrics, this.updateLastTipsMetric);
        registerHandler(WSMsgType.ComponentCounterMetrics, this.updateLastComponentMetric);
        this.updateCollecting(true);
    }

    unregisterHandlers = () => {
        unregisterHandler(WSMsgType.Status);
        unregisterHandler(WSMsgType.MPSMetrics);
        unregisterHandler(WSMsgType.NeighborStats);
        unregisterHandler(WSMsgType.TipsMetrics);
        unregisterHandler(WSMsgType.ComponentCounterMetrics);
        this.updateCollecting(false);
    }

    @action
    updateCollecting = (collecting: boolean) => {
        this.collecting = collecting;
    }

    @action
    reset() {
        this.collected_mps_metrics = [];
        this.collected_mem_metrics = [];
        this.neighbor_metrics = new ObservableMap<string, NeighborMetrics>();
        this.collected_tips_metrics = [];
        this.collected_component_counter_metrics = [];
    }

    reconnect() {
        this.updateWebSocketConnected(false);
        setTimeout(() => {
            this.connect();
        }, 5000);
    }

    connect() {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.reconnect(),
            () => this.updateWebSocketConnected(false))
    }

    @action
    updateWebSocketConnected = (connected: boolean) => this.websocketConnected = connected;

    @action
    updateStatus = (status: Status) => {
        status.mem.ts = dateformat(Date.now(), "HH:MM:ss");
        if (this.collected_mem_metrics.length > maxMetricsDataPoints) {
            this.collected_mem_metrics.shift();
        }
        this.collected_mem_metrics.push(status.mem);
        this.status = status;
    };

    @action
    updateNeighborMetrics = (neighborMetrics: Array<NeighborMetric>) => {
        if (!neighborMetrics) {
            return;
        }
        let updated = [];
        for (let i = 0; i < neighborMetrics.length; i++) {
            let metric = neighborMetrics[i];
            let neighbMetrics: NeighborMetrics = this.neighbor_metrics.get(metric.id);
            if (!neighbMetrics) {
                neighbMetrics = new NeighborMetrics();
            }
            neighbMetrics.addMetric(metric);
            this.neighbor_metrics.set(metric.id, neighbMetrics);
            updated.push(metric.id);
        }
        // remove duplicates
        for (const k of this.neighbor_metrics.keys()) {
            if (!updated.includes(k)) {
                this.neighbor_metrics.delete(k);
            }
        }
    };

    @action
    updateLastMPSMetric = (mps: number) => {
        let mpsMetric = new MPSMetric();
        mpsMetric.mps = mps;
        mpsMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_mps_metric = mpsMetric;
        return mpsMetric;
    };

    @action
    addMPSMetric = (metric: MPSMetric) => {
        if (this.collected_mps_metrics.length > maxMetricsDataPoints) {
            this.collected_mps_metrics.shift();
        }
        this.collected_mps_metrics.push(metric);
    }

    @action
    updateLastTipsMetric = (tipsMetric: TipsMetric) => {
        tipsMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_tips_metric = tipsMetric;
        if (this.collected_tips_metrics.length > maxMetricsDataPoints) {
            this.collected_tips_metrics.shift();
        }
        this.collected_tips_metrics.push(tipsMetric);
    };

    @action
    updateLastComponentMetric = (componentCounterMetric: ComponentCounterMetric) => {
        componentCounterMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_component_counter_metric = componentCounterMetric;
        if (this.collected_component_counter_metrics.length > maxMetricsDataPoints) {
            this.collected_component_counter_metrics.shift()
        }
        this.collected_component_counter_metrics.push(componentCounterMetric);
    };

    @computed
    get mpsSeries() {
        let mps = Object.assign({}, chartSeriesOpts,
            series("MPS", 'rgba(67, 196, 99,1)', 'rgba(67, 196, 99,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_mps_metrics.length; i++) {
            let metric: MPSMetric = this.collected_mps_metrics[i];
            labels.push(metric.ts);
            mps.data.push(metric.mps);
        }

        return {
            labels: labels,
            datasets: [mps],
        };
    }

    @computed
    get tipsSeries() {
        let totaltips = Object.assign({}, chartSeriesOpts,
            series("All tips", 'rgba(67, 196, 99,1)', 'rgba(67, 196, 99,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tips_metrics.length; i++) {
            let metric: TipsMetric = this.collected_tips_metrics[i];
            labels.push(metric.ts);
            totaltips.data.push(metric.totaltips);
        }

        return {
            labels: labels,
            datasets: [totaltips],
        };
    }

    @computed
    get componentSeries() {
        let stored = Object.assign({}, chartSeriesOpts,
            series("stored", 'rgba(209,165,253,1)', 'rgba(209,165,253,0.4)')
        );
        let solidified = Object.assign({}, chartSeriesOpts,
            series("solidified", 'rgba(165,209,253,1)', 'rgba(165,209,253,0.4)')
        );
        let scheduled = Object.assign({}, chartSeriesOpts,
            series("scheduled", 'rgba(182, 141, 64,1)', 'rgba(182, 141, 64,0.4)')
        );
        let booked = Object.assign({}, chartSeriesOpts,
            series("booked", 'rgba(5, 68, 94,1)', 'rgba(5, 68, 94,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_component_counter_metrics.length; i++) {
            let metric: ComponentCounterMetric = this.collected_component_counter_metrics[i];
            labels.push(metric.ts);
            stored.data.push(metric.store);
            solidified.data.push(metric.solidifier);
            scheduled.data.push(metric.scheduler);
            booked.data.push(metric.booker);
        }

        return {
            labels: labels,
            datasets: [stored, solidified, scheduled, booked],
        };
    }

    @computed
    get neighborsSeries() {
        return {};
    }

    @computed
    get uptime() {
        let day, hour, minute, seconds;
        seconds = Math.floor(this.status.uptime / 1000);
        minute = Math.floor(seconds / 60);
        seconds = seconds % 60;
        hour = Math.floor(minute / 60);
        minute = minute % 60;
        day = Math.floor(hour / 24);
        hour = hour % 24;
        let str = "";
        if (day == 1) {
            str += day + " Day, ";
        }
        if (day > 1) {
            str += day + " Days, ";
        }
        if (hour >= 0) {
            if (hour < 10) {
                str += "0" + hour + ":";
            } else {
                str += hour + ":";
            }
        }
        if (minute >= 0) {
            if (minute < 10) {
                str += "0" + minute + ":";
            } else {
                str += minute + ":";
            }
        }
        if (seconds >= 0) {
            if (seconds < 10) {
                str += "0" + seconds;
            } else {
                str += seconds;
            }
        }

        return str;
    }

    @computed
    get memSeries() {
        let heapSys = Object.assign({}, chartSeriesOpts,
            series("Heap Sys", 'rgba(168, 50, 76,1)', 'rgba(168, 50, 76,0.4)')
        );
        let heapAlloc = Object.assign({}, chartSeriesOpts,
            series("Heap Alloc", 'rgba(222, 49, 87,1)', 'rgba(222, 49, 87,0.4)')
        );
        let heapIdle = Object.assign({}, chartSeriesOpts,
            series("Heap Idle", 'rgba(222, 49, 182,1)', 'rgba(222, 49, 182,0.4)')
        );
        let heapReleased = Object.assign({}, chartSeriesOpts,
            series("Heap Released", 'rgba(250, 76, 252,1)', 'rgba(250, 76, 252,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_mem_metrics.length; i++) {
            let metric = this.collected_mem_metrics[i];
            labels.push(metric.ts);
            heapSys.data.push(metric.heap_sys);
            heapAlloc.data.push(metric.heap_alloc);
            heapIdle.data.push(metric.heap_idle);
            heapReleased.data.push(metric.heap_released);
        }

        return {
            labels: labels,
            datasets: [heapSys, heapAlloc, heapIdle, heapReleased],
        };
    }
}

export default NodeStore;
