import * as React from 'react';
import Button from 'react-bootstrap/Button';
import GlobalStore from '../stores/GlobalStore';
import { inject, observer } from 'mobx-react';
import moment, { isMoment, Moment } from 'moment';
import Datetime from 'react-datetime';
import { DATE_FORMAT, TIME_FORMAT } from '../utils/constants';

interface Props {
    globalStore?: GlobalStore;
    isStartTime: boolean;
}

@inject('globalStore')
@observer
export class TimePickerButtons extends React.Component<Props, any> {
    setCurrent = () => {
        const m = moment();
        this.updateSearchTimes(m);
    };

    addTime = () => {
        let m = this.props.isStartTime
            ? moment.unix(this.props.globalStore.searchStartingTime)
            : moment.unix(this.props.globalStore.searchEndingTime);
        m = m.add(30, 'seconds');
        this.updateSearchTimes(m);
    };

    updateSearchTimes = (m: Moment) => {
        if (this.props.isStartTime) {
            this.props.globalStore.updateSearchStartingTime(m);
            this.props.globalStore.updateStartManualPicker(true);
        } else {
            this.props.globalStore.updateSearchEndingTime(m);
            this.props.globalStore.updateEndManualPicker(true);
        }
    };

    subTime = () => {
        let m = this.props.isStartTime
            ? moment.unix(this.props.globalStore.searchStartingTime)
            : moment.unix(this.props.globalStore.searchEndingTime);
        m = m.subtract(30, 'seconds');
        this.updateSearchTimes(m);
    };

    render() {
        return (
            <div>
                <div className={'tripleButton'}>
                    <Button
                        className={'button button-wide button-left'}
                        variant="outline-secondary"
                        onClick={this.setCurrent}
                    >
                        Set current time
                    </Button>
                    <Button
                        className={'button button-middle'}
                        variant="outline-secondary"
                        onClick={this.subTime}
                    >
                        -30s
                    </Button>
                    <Button
                        className={'button button-right'}
                        variant="outline-secondary"
                        onClick={this.addTime}
                    >
                        +30s
                    </Button>
                </div>
            </div>
        );
    }
}

interface PickerProps {
    globalStore?: GlobalStore;
    isStartTime?: boolean;
    dateTime?: Moment | undefined;
    onOpenFunc?: any;
}

export class Picker extends React.Component<PickerProps, any> {
    updateFrom = (date) => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchStartingTime(date);
        }
    };

    updateTo = (date) => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchEndingTime(date);
        }
    };

    render() {
        const isStartTime = this.props.isStartTime;
        const updateFunc = isStartTime ? this.updateFrom : this.updateTo;
        const onOpenFunc = this.props.onOpenFunc;
        return (
            <>
                <Datetime
                    dateFormat={DATE_FORMAT}
                    timeFormat={TIME_FORMAT}
                    onChange={updateFunc}
                    value={this.props.dateTime}
                    onOpen={onOpenFunc}
                />
            </>
        );
    }
}
