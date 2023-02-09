import * as React from 'react';
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class EpochInfo extends React.Component<Props, any> {

}
