import {Output, SigLockedColoredOutput, SigLockedSingleOutput} from "app/misc/Payload";
import { SigLockedSingleOutputComponent} from "app/components/SigLockedSingleOutputComponent";
import * as React from "react";
import {SigLockedColoredOutputComponent} from "app/components/SigLockedColoredOutputComponent";

export function outputToComponent(output: Output) {
    switch (output.type) {
        case "SigLockedSingleOutputType":
            return <SigLockedSingleOutputComponent output={output.output as SigLockedSingleOutput} id={output.outputID} />;
        case "SigLockedColoredOutputType":
            return <SigLockedColoredOutputComponent output={output.output as SigLockedColoredOutput} id={output.outputID} />;
        case "AliasOutputType":
            return;
        case "ExtendedLockedOutputType":
            return;
        default:
            return;
    }
}