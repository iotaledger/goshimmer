import * as React from 'react';
import {KeyboardEvent} from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import FormControl from "react-bootstrap/FormControl";
import ChatStore from "app/stores/ChatStore";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import InputGroup from "react-bootstrap/InputGroup";

interface Props {
    nodeStore?: NodeStore;
    chatStore?: ChatStore;
}

@inject("nodeStore")
@inject("chatStore")
@observer
export class ChatSend extends React.Component<Props, any> {
    constructor(props) {
      super(props);
      this.state = {
        chatBlock: '', // the block
        error: false, // if there was an error or not
        infoBlock: null // something to display to the user, like Sent! or Booh!
      };
    }
    timeout = null; // just a timeout to remove the block from the screen after N seconds
    updateSend = (e) => {
      //Update the State
      this.setState({
        chatBlock: e.target.value
      });
    };
    sendBlock = async (e: KeyboardEvent) => {
      if (e.key !== 'Enter' || this.state.chatBlock === '') return;
      this.props.chatStore
        .sendBlock(this.state.chatBlock)
        .then(() => {
          // if all good, we reset the input field and display a success block
          this.setState({
            chatBlock: '',
            infoBlock: 'Block sent!',
            error: false
          });
          // but then we remove the block from the screen, just a personal thing
          this.timeout = setTimeout(() => {
            this.setState({
              infoBlock: null,
              error: false
            });
          }, 10000);
        })
        .catch((e) => {
          // if there was an error, we dont clean the input and display an error block
          this.setState({
            infoBlock: 'Ooops error!',
            error: true
          });
          // and then we remove the block from the screen, for error blocks perhaps is not necessary
          this.timeout = setTimeout(() => {
            this.setState({
              infoBlock: null,
              error: false
            });
          }, 10000);
        });
    };
    componentWillUnmount() {
      if (this.timeout) {
        clearTimeout(this.timeout); // just in case we clear the timeout when the component will unmount
      }
    }
    render() {
      let { sending } = this.props.chatStore;
      return (
        <React.Fragment>
          <Row className={'mb-3'}>
            <Col>
              <h6>Send a block via the Tangle</h6>
              <InputGroup className="mb-3">
                <FormControl
                  placeholder="Send Block"
                  aria-label="Send Block"
                  aria-describedby="basic-addon1"
                  value={this.state.chatBlock}
                  onChange={this.updateSend}
                  onKeyUp={this.sendBlock}
                  disabled={sending}
                  maxlength="1000"
                />
              </InputGroup>
              {this.state.infoBlock && (
                <p style={{ color: `${this.state.error ? 'red' : 'inherit'}` }}>
                  {this.state.infoBlock}
                </p>
              )}
            </Col>
          </Row>
        </React.Fragment>
      );
    }
  }