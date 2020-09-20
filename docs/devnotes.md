## Relay Server Protocol

Connect -> Returns Messages
Slot -> Creates a new slot or attaches to existing slot for relaying
  * Once slots are filled it starts passing messages between the connections
  
Relay Server Design
   for {
        connection := Accept Connection
        go HandleConnection(connection)
   }
   
func HandleConnection(connection) {
    Authenticate(connection)
    SendMessages(connection)
    for {
        command := Receive(connection)
        switch command {
        case "messages":
              SendMessages(connection)
        case "slot"
             slot := getOrCreateSlot()
             if slot.isFull {
                  go connectSlots(connection, slot)
                  break
             }
             sendSlotInfo(connection, slot)
    }        
}