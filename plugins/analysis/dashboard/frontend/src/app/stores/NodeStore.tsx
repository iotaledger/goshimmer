import {action, computed, observable, ObservableMap} from 'mobx';
import * as dateformat from 'dateformat';
import {connectWebSocket, registerHandler, unregisterHandler, WSMsgType} from "app/misc/WS";

class MPSMetric {
    mps: number;
    ts: string;
}

class Status {
    id: string;
    version: string;
    uptime: number;
    mem: MemoryMetrics = new MemoryMetrics();
}

class MemoryMetrics {
    sys: number;
    heap_sys: number;
    heap_inuse: number;
    heap_idle: number;
    heap_released: number;
    heap_objects: number;
    m_span_inuse: number;
    m_cache_inuse: number;
    stack_sys: number;
    last_pause_gc: number;
    num_gc: number;
    ts: string;
}

class TipsMetric {
    tips: number;
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
                tx: this.current.bytes_written - this.secondLast.bytes_written,
                rx: this.current.bytes_read - this.secondLast.bytes_read,
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
    bytes_read: number;
    bytes_written: number;
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
    @observable collecting: boolean = true;

    constructor() {
        this.registerHandlers();
    }

    registerHandlers = () => {
        registerHandler(WSMsgType.Status, this.updateStatus);
        registerHandler(WSMsgType.MPSMetrics, (mps: number) => {
            this.addMPSMetric(this.updateLastMPSMetric(mps));
        });
        registerHandler(WSMsgType.NeighborStats, this.updateNeighborMetrics);
        registerHandler(WSMsgType.TipsMetrics, this.updateLastTipsMetric);
        this.updateCollecting(true);
    }

    unregisterHandlers = () => {
        unregisterHandler(WSMsgType.Status);
        registerHandler(WSMsgType.MPSMetrics, this.updateLastMPSMetric);
        unregisterHandler(WSMsgType.NeighborStats);
        unregisterHandler(WSMsgType.TipsMetrics);
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
    }

    connect() {
        connectWebSocket(statusWebSocketPath,
            () => this.updateWebSocketConnected(true),
            () => this.updateWebSocketConnected(false),
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
    updateLastTipsMetric = (tips: number) => {
        let tipsMetric = new TipsMetric();
        tipsMetric.tips = tips;
        tipsMetric.ts = dateformat(Date.now(), "HH:MM:ss");
        this.last_tips_metric = tipsMetric;
        if (this.collected_tips_metrics.length > maxMetricsDataPoints) {
            this.collected_tips_metrics.shift();
        }
        this.collected_tips_metrics.push(tipsMetric);
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
        let tips = Object.assign({}, chartSeriesOpts,
            series("Tips", 'rgba(250, 140, 30,1)', 'rgba(250, 140, 30,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_tips_metrics.length; i++) {
            let metric: TipsMetric = this.collected_tips_metrics[i];
            labels.push(metric.ts);
            tips.data.push(metric.tips);
        }

        return {
            labels: labels,
            datasets: [tips],
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
        let heapAlloc = Object.assign({}, chartSeriesOpts,
            series("Heap Alloc", 'rgba(168, 50, 76,1)', 'rgba(168, 50, 76,0.4)')
        );
        let heapInuse = Object.assign({}, chartSeriesOpts,
            series("Heap In-Use", 'rgba(222, 49, 87,1)', 'rgba(222, 49, 87,0.4)')
        );
        let heapIdle = Object.assign({}, chartSeriesOpts,
            series("Heap Idle", 'rgba(222, 49, 182,1)', 'rgba(222, 49, 182,0.4)')
        );
        let heapReleased = Object.assign({}, chartSeriesOpts,
            series("Heap Released", 'rgba(250, 76, 252,1)', 'rgba(250, 76, 252,0.4)')
        );
        let stackAlloc = Object.assign({}, chartSeriesOpts,
            series("Stack Alloc", 'rgba(54, 191, 173,1)', 'rgba(54, 191, 173,0.4)')
        );
        let sys = Object.assign({}, chartSeriesOpts,
            series("Total Alloc", 'rgba(160, 50, 168,1)', 'rgba(160, 50, 168,0.4)')
        );

        let labels = [];
        for (let i = 0; i < this.collected_mem_metrics.length; i++) {
            let metric = this.collected_mem_metrics[i];
            labels.push(metric.ts);
            heapAlloc.data.push(metric.heap_sys);
            heapInuse.data.push(metric.heap_inuse);
            heapIdle.data.push(metric.heap_idle);
            heapReleased.data.push(metric.heap_released);
            stackAlloc.data.push(metric.stack_sys);
            sys.data.push(metric.sys);
        }

        return {
            labels: labels,
            datasets: [sys, heapAlloc, heapInuse, heapIdle, heapReleased, stackAlloc],
        };
    }
}

export default NodeStore;
