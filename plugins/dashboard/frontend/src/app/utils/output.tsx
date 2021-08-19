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
import {ExplorerOutput, GoF} from "app/stores/ExplorerStore";
import {Base58EncodedColorIOTA, resolveColor} from "app/utils/color";

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

export function totalBalanceFromExplorerOutputs(outputs: Array<ExplorerOutput>, addy: string): Map<string, number> {
    let totalBalance: Map<string,number> = new Map();
    if (outputs.length === 0) {return totalBalance;}
    for (let i = 0; i < outputs.length; i++) {
        let o = outputs[i];
        if (o.metadata.gradeOfFinality !== GoF.High) {
            // ignore all unconfirmed balances
            continue
        }
        switch (o.output.type) {
            case "SigLockedSingleOutputType":
                let single = o.output.output as SigLockedSingleOutput;
                let resolvedColor = resolveColor(Base58EncodedColorIOTA);
                let prevBalance = totalBalance.get(resolvedColor);
                if (prevBalance === undefined) {prevBalance = 0;}
                totalBalance.set(resolvedColor, single.balance + prevBalance);
                break;
            case "ExtendedLockedOutputType":
                let extended = o.output.output as ExtendedLockedOutput;
                if (extended.fallbackAddress === undefined) {
                    // no fallback addy, address controls the output
                    extractBalanceInfo(o, totalBalance);
                    break;
                } else {
                    let now = new Date().getTime()/1000;
                    // there is a fallback address, it it us?
                    if (extended.fallbackAddress === addy) {
                        // our address is the fallback
                        // check if fallback deadline expired
                        if (now > extended.fallbackDeadline) {
                            extractBalanceInfo(o, totalBalance);
                            break;
                        }
                        // we have the fallback addy and fallback hasn't expired yet, balance not available
                        break;
                    }
                    // means addy can only be the original address
                    // we own the balance if we are before the deadline
                    if (now < extended.fallbackDeadline) {
                        extractBalanceInfo(o, totalBalance);
                        break;
                    }
                }
                break;
            default:
                if (o.output.output.balances === null) {return;}
                extractBalanceInfo(o, totalBalance);
        }
    }
    return totalBalance
}

let extractBalanceInfo = (o: ExplorerOutput, result: Map<string, number>) => {
    let colorKeys = Object.keys(o.output.output.balances);
    for (let i = 0; i< colorKeys.length; i++) {
        let color = colorKeys[i];
        let balance = o.output.output.balances[color];
        let resolvedColor = resolveColor(color);
        let prevBalance = result.get(resolvedColor);
        if (prevBalance === undefined) {
            prevBalance = 0;
        }
        result.set(resolvedColor, balance + prevBalance);
    }
}