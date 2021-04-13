import {
    AliasOutput,
    ExtendedLockedOutput,
    Output,
    SigLockedColoredOutput,
    SigLockedSingleOutput
} from "app/misc/Payload";
import { SigLockedSingleOutputComponent} from "app/components/SigLockedSingleOutputComponent";
import * as React from "react";
import {SigLockedColoredOutputComponent} from "app/components/SigLockedColoredOutputComponent";
import {AliasOutputComponent} from "app/components/AliasOutputComponent.tsx";
import {ExtendedLockedOutputComponent} from "app/components/ExtendedLockedOutput";

export function outputToComponent(output: Output) {
    switch (output.type) {
        case "SigLockedSingleOutputType":
            return <SigLockedSingleOutputComponent output={output.output as SigLockedSingleOutput} id={output.outputID}/>;
        case "SigLockedColoredOutputType":
            return <SigLockedColoredOutputComponent output={output.output as SigLockedColoredOutput} id={output.outputID}/>;
        case "AliasOutputType":
            return <AliasOutputComponent output={output.output as AliasOutput} id={output.outputID}/>;
        case "ExtendedLockedOutputType":
            return <ExtendedLockedOutputComponent output={output.output as ExtendedLockedOutput} id={output.outputID}/>;
        default:
            return;
    }
}