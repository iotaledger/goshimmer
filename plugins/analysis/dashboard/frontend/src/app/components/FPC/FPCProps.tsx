import FPCStore from "app/stores/FPCStore";
import { RouteComponentProps } from "react-router";

export interface FPCProps extends RouteComponentProps<{
    id?: string;
}> {
    fpcStore?: FPCStore;
}
