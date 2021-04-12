import {Output, SigLockedColoredOutput, SigLockedSingleOutput} from "app/misc/Payload";
import { SigLockedSingleOutputComponent} from "app/components/SigLockedSingleOutputComponent";
import * as React from "react";
import {SigLockedColoredOutputComponent} from "app/components/SigLockedColoredOutputComponent";

export function outputToComponent(output: Output) {
    switch (output.type) {
        case "SigLockedSingleOutput":
            return <SigLockedSingleOutputComponent output={output.output as SigLockedSingleOutput} id={output.output_id} />;
        case "SigLockedColoredOutput":
            return <SigLockedColoredOutputComponent output={output.output as SigLockedColoredOutput} id={output.output_id} />;
        case "AliasOutput":
            return;
        case "ExtendedLockedOutput":
            return;
        default:
            return;
    }
}