import RouterStore from "app/stores/RouterStore";
import AutopeeringStore from "app/stores/AutopeeringStore";

export interface RootProps {
    history: any;
    routerStore?: RouterStore;
    autopeeringStore?: AutopeeringStore;
}
