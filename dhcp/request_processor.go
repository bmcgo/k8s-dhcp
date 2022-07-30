package dhcp

import (
	"errors"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"net"
)

const dhcpRequestChanBufSize = 1024

type ResponseGetter func(req Request) (Response, error)

type RequestProcessor struct {
	socket             Socket
	dhcpRequestChan    chan Request
	server             *Server
	callbackSaveLeases CallbackSaveLeases
	log                RLogger
}

func NewRequestProcessor(listen Listen,
	socketFactory SocketFactory,
	callbackSaveLeases CallbackSaveLeases,
	server *Server,
	logger RLogger) (*RequestProcessor, error) {
	var err error
	listenerName := fmt.Sprintf("listener[%s]", listen.ToString())
	l := &RequestProcessor{
		dhcpRequestChan:    make(chan Request, dhcpRequestChanBufSize),
		callbackSaveLeases: callbackSaveLeases,
		log:                logger.WithName(listenerName),
		server:             server,
	}
	l.socket, err = socketFactory(listen.Addr, listen.Interface, logger)
	if err != nil {
		return nil, err
	}
	l.startRequestProcessors()
	return l, nil
}

func (s *RequestProcessor) runResponseProcessor(responseChan <-chan Response) {
	var response Response
	var responses []Response
	var err error
	var more bool
	for {
		select {
		case response, more = <-responseChan:
			if !more {
				return
			}
			responses = append(responses, response)
			s.log.Debugf("added offer response to queue")
		default:
			if len(responses) > 0 {
				s.log.Debugf("saving offers (%d)", len(responses))
				err = s.callbackSaveLeases(responses)
				if err != nil {
					s.log.Errorf(err, "failed to save %d offers", len(responses))
					responses = []Response{}
					break
				}
				for _, response = range responses {
					err = response.Send()
					if err != nil {
						s.log.Errorf(err, "failed to send response: %s", response)
					}
				}
			} else {
				s.log.Debugf("empty response queue")
			}
			response, more = <-responseChan
			if !more {
				return
			}
			responses = []Response{response}
		}
	}
}

func (s *RequestProcessor) runRequestProcessor() {
	var (
		req  Request
		resp Response
		err  error
		more bool
	)
	responseChan := make(chan Response, 1024)

	go s.runResponseProcessor(responseChan)
	s.log.Debugf("Started worker")
	for {
		req, more = <-s.dhcpRequestChan
		if !more {
			s.log.Infof("No more packets. Exiting worker")
			close(responseChan)
			return
		}
		resp, err = s.server.GetResponse(req)
		if err != nil {
			s.log.Errorf(err, "Failed to get response to request: %s", req.String())
			continue
		} else {
			//TODO send NAK and continue
			switch resp.Response.MessageType() {
			case dhcpv4.MessageTypeOffer:
				resp.Lease.AckSent = false
				responseChan <- resp
			case dhcpv4.MessageTypeAck:
				resp.Lease.AckSent = true
				responseChan <- resp
			default:
				s.log.Infof("unknown response type: %s", resp.Response.String())
			}
		}
	}
}

func (s *RequestProcessor) startRequestProcessors() {
	go s.runRequestProcessor()
}

func (s *RequestProcessor) Serve() error {
	for {
		req, err := s.socket.NextRequest()
		if err != nil {
			e, ok := errors.Unwrap(err).(*net.OpError)
			if ok {
				if e.Err == net.ErrClosed {
					s.log.Infof("Connection closed. Stopping server.")
					return nil
				}
			}
			s.log.Errorf(err, "Error reading packet")
			if !e.Temporary() {
				return e
			}
		} else {
			s.dhcpRequestChan <- *req
		}
	}
}

func (s *Response) Send() error {
	if isAddressZero(s.Request.GatewayIPAddr) {
		return s.Request.socket.SendBroadcast(s.Request, s.Response)
	}
	return s.Request.socket.SendResponse(s.Request, s.Response)
}

func (s *RequestProcessor) Close() {
	s.socket.Close()
	//TODO: close responders
}
