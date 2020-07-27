import { AutopeeringStore } from "./stores/AutopeeringStore";
import { RouteComponentProps } from "react-router";

export interface AppProps extends RouteComponentProps {
    autopeeringStore: AutopeeringStore;
}
