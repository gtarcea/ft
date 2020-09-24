package ft

import (
	"context"
	"net"
	"time"
)

type serviceDiscovererLoop struct {
	sd *ServiceDiscoverer
}

func (s *serviceDiscovererLoop) broadcastLoop(broadcastAddr *net.UDPAddr) {
	// Need our starting time so we can exit after searching for services for s.TimeLimit
	startingTime := time.Now()

	// Loop sending out broadcasts.
BroadcastLoop:
	for {
		// Send payload out on the multicast address/port
		s.broadcast(broadcastAddr)

		// There are a number of conditions that determine if we should stop searching for
		// services:

		// 1. Stop if the user defined duration for finding services has been reached
		if broadcastTimeLimitReached(startingTime, s.sd.TimeLimit) {
			break
		}

		select {
		case <-s.sd.ctx.Done():
			// 2. Stop if the ctx is done (cancel function called on context)
			break BroadcastLoop

		case <-time.After(s.sd.BroadcastDelay): // Wait for the broadcast delay time before looping again
		}
	}
}

func (s *serviceDiscovererLoop) collectLoop() ([]Service, error) {
	return s.broadcastAnCollectLoop(nil)
}

// broadcastAndCollectLoop will start up a background go routine to collect responses to its broadcasts. It will then
// enter a loop sending out broadcasts of payload (ServiceDiscoverer payload) on the multicast address and port.
func (s *serviceDiscovererLoop) broadcastAnCollectLoop(broadcastAddr *net.UDPAddr) ([]Service, error) {
	// Create an context so we can tell the listener go routine to stop.
	ctx, cancelCollection := context.WithCancel(context.Background())

	// Start the collector in the background
	serviceCollector, err := s.startResponseCollectorInBackground(ctx)
	if err != nil {
		return nil, err
	}

	// Need our starting time so we can exit after searching for services for s.TimeLimit
	startingTime := time.Now()

	// Loop sending out broadcasts.
	// The serviceCollector will collect the responses to the broadcasts.
BroadcastAndCollectLoop:
	for {
		// Send payload out on the multicast address/port if there is an address to broadcast on.
		// s.collectLoop() calls broadcastAndCollectLoop(nil) (passes nil for broadcastAddr), this
		// allows for re-using essentially the same code where the only difference is that collectLoop()
		// would not call broadcast. So we check for null in this one spot to facilitate this reuse.
		if broadcastAddr != nil {
			s.broadcast(broadcastAddr)
		}

		// There are a number of conditions that determine if we should stop searching for
		// services:

		// 1. Stop if the user defined duration for finding services has been reached
		if discoveryTimeLimitReached(startingTime, s.sd.TimeLimit) {
			break
		}

		// 2. Stop if the listener go routine has collected the user defined maximum number of
		// service responses.
		if serviceCollector.maxServicesFound {
			break
		}

		select {
		case <-s.sd.ctx.Done():
			// 3. Stop if the ctx is done (cancel function called on context)
			break BroadcastAndCollectLoop

		case <-time.After(s.sd.BroadcastDelay): // Wait for the delay time before looping again
		}
	}

	// At this point we've stopped broadcasting. So now we tell the service collector to shutdown
	// and then wait for it to stop
	cancelCollection()
	serviceCollector.wg.Wait()

	return serviceCollector.toServicesList(), nil
}

// startResponseCollectorInBackground starts a serviceCollector running in the background listening
// for responses to the broadcast. The context is used to tell the background listener to exit.
func (s *serviceDiscovererLoop) startResponseCollectorInBackground(ctx context.Context) (*serviceCollector, error) {
	// The service collector will collect all the responses to the broadcast. It shares much of the setup
	// information with the ServiceDiscoverer so rather than duplicate that state we can just share it.
	serviceCollector := &serviceCollector{
		sd:        s.sd,
		responses: make(map[string][]byte),
	}

	// Make sure that we can actually create the network connection that the listener go routine
	// will listen on.
	conn, err := serviceCollector.createConnection()
	if err != nil {
		return nil, err
	}

	// Start up a listener that will collect the responses from services that respond to the broadcast. Since
	// broadcastAndCollectLoop() needs to know when the listener has exited we use a wait group to synchronize on.
	serviceCollector.wg.Add(1)
	go serviceCollector.listenForAndCollectResponses(conn, ctx)

	return serviceCollector, nil
}

// broadcast will write payload to the multicast address for all the host interfaces.
func (s *serviceDiscovererLoop) broadcast(dst *net.UDPAddr) {
	for _, iface := range s.sd.interfaces {
		if err := s.sd.packetConn.SetMulticastInterface(&iface); err != nil {
			// log error
			continue
		}

		_ = s.sd.packetConn.SetMulticastTTL(2)
		if _, err := s.sd.packetConn.WriteTo(s.sd.payload, dst); err != nil {
			// log error
		}
	}
}

// discoveryTimeLimitReached returns true if the time limit for service discovery has been reached.
func discoveryTimeLimitReached(startingTime time.Time, durationLimit time.Duration) bool {
	if durationLimit < 0 {
		return false
	}

	return time.Since(startingTime) > durationLimit
}

func broadcastTimeLimitReached(startTime time.Time, durationLimit time.Duration) bool {
	return discoveryTimeLimitReached(startTime, durationLimit)
}
