import {
    AliasOutput,
    ExtendedLockedOutput,
    Output,
    OutputID,
    SigLockedColoredOutput,
    SigLockedSingleOutput
} from "app/misc/Payload";
import { SigLockedSingleOutputComponent} from "app/components/SigLockedSingleOutputComponent";
import * as React from "react";
import {SigLockedColoredOutputComponent} from "app/components/SigLockedColoredOutputComponent";
import {AliasOutputComponent} from "app/components/AliasOutputComponent.tsx";
import {ExtendedLockedOutputComponent} from "app/components/ExtendedLockedOutput";
import {ExplorerOutput} from "app/stores/ExplorerStore";
import {Base58EncodedColorIOTA, resolveColor} from "app/utils/color";
import {ConfirmationState} from "app/utils/confirmation_state";
import { Base58, ReadStream } from '@iota/util.js';

export function outputToComponent(output: Output) {
    const id = outputIDFromBase58(output.outputID.base58);
    switch (output.type) {
        case "SigLockedSingleOutputType":
            return <SigLockedSingleOutputComponent output={output.output as SigLockedSingleOutput} id={id}/>;
        case "SigLockedColoredOutputType":
            return <SigLockedColoredOutputComponent output={output.output as SigLockedColoredOutput} id={id}/>;
        case "AliasOutputType":
            return <AliasOutputComponent output={output.output as AliasOutput} id={id}/>;
        case "ExtendedLockedOutputType":
            return <ExtendedLockedOutputComponent output={output.output as ExtendedLockedOutput} id={id}/>;
        default:
            return;
    }
}

export function totalBalanceFromExplorerOutputs(outputs: Array<ExplorerOutput>, addy: string): Map<string, number> {
    let totalBalance: Map<string,number> = new Map();
    if (outputs.length === 0) {return totalBalance;}
    for (let i = 0; i < outputs.length; i++) {
        let o = outputs[i];
        if (o.metadata.confirmationState < ConfirmationState.Accepted) {
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

export function outputIDFromBase58(outputIDStr: string): OutputID {
    const outputIDBytes = Base58.decode(outputIDStr);

    const readStream = new ReadStream(outputIDBytes);
    return {
        base58: outputIDStr,
        transactionID: Base58.encode(readStream.readBytes('TransactionID', 32)),
        outputIndex: Number(readStream.readUInt16('Index')),
    };
}