import * as React from 'react';
import { inject, observer } from 'mobx-react';
import GlobalStore from 'stores/GlobalStore';

interface Props {
    globalStore?: GlobalStore;
    route: string;
    title: string;
}

@inject('globalStore')
@observer
export default class LinkToDashboard extends React.Component<Props, any> {
    render() {
        const { route, title } = this.props;
        const { explorerAddress } = this.props.globalStore;

        return (
            <a
                href={`${explorerAddress}/${route}`}
                target="_blank"
                rel="noopener noreferrer"
            >
                {title}
            </a>
        );
    }
}
