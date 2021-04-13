# Test of Booker and Marker Mapping


The Marker tool is tested for the booker with the following scenario:

![](https://imgur.com/sCkXXrG.png)


We apply the following defintions:
- a colored field indicates that a value is set
- a field without color indicates the values are inherited
- [.] for a Branch indicates inheritance

The transaction-by-transaction development of the data associated with the marker DAG is as follows:

### Transaction 1 (create Marker): 

- message 1: 
    - create sequence ID 1, for now sequence ID of Main = 1
    - PastMarker.BranchID = MainBranchID
    - Payload.BranchID = MainBranchID

Note that MainBranchID does not own sequence ID 1, although the PastMarker has sequence ID 1.

![](https://imgur.com/Wsobgv0.png)

### Transaction 2 (create Marker, double spend): 

- message 2 double spends message 1
- message 1:
    - create Branch A
    - Branch A gets sequence ID = 1
    - PastMarker.BranchID = A
    - Payload.BranchID = A
- message 2: 
    - create Branch B
    - create sequence ID 2
    - PastMarker.BranchID = B
    - Payload.BranchID = B

![](https://imgur.com/9SqyjlY.png)

### Transaction 3 (create Marker, double spend): 

- message 3 double spends message 1
- message 3 
    - create Branch C
    - create sequence ID 3
    - PastMarker.BranchID = C
    - Payload.BranchID = C

![](https://imgur.com/N8xffwD.png)

### Transaction 4 (create Marker): 

- message 4
    - create sequence ID 4, for now sequence ID of Main = 4
    - PastMarker.BranchID = Main
    - Payload.BranchID = Main

![](https://imgur.com/GIJjo0b.png)

### Transaction 5 (create Marker): 

- message 5
    - PastMarker.BranchID = Main
    - Payload.BranchID = Main

![](https://imgur.com/YMGAbiF.png)

### Transaction 6 (create Marker): 

- message 6
    - PastMarker.BranchID = [A], [.] indicates inheritance
    - Payload.BranchID = Main, since no double spend

![](https://imgur.com/ZtSYkvq.png)

### Transaction 7 (create Marker): 

- message 7
    - PastMarker.BranchID = [C], [.] indicates indirect mapping of the marker to the Branch
    - Payload.BranchID = Main, since no double spend

![](https://imgur.com/eqmTCnY.png)

### Transaction 8 (create Marker): 

- message 8
    - PastMarker.BranchID = [A], [.] indicates indirect mapping of the marker to the Branch, 
    - Payload.BranchID = Main, since no double spend

![](https://imgur.com/gbLlGgd.png)

### Transaction 9 (create Marker, double spend): 

- message 9
    - PastMarker.BranchID = [C], [.] indicates indirect mapping of the marker to the Branch
    - Payload.BranchID = Main, since no double spend

![](https://imgur.com/i6ZFvnF.png)

### Transaction 10 (create Marker, double spend): 

- message 5
    - create Branch D
    - change: PastMarker.BranchID = D
    - change: Payload.BranchID = D
- message 10
    - double spends message 5
    - create Branch E
    - PastMarker.BranchID = B+E
    - Payload.BranchID = E

![](https://imgur.com/lImMAQY.png)

### Transaction 11 (create Marker): 

- Message 11: 
    - PastMarker.BranchID = A+C
    - Payload.BranchID = Main

![](https://imgur.com/WT0qEEH.png)

### Transaction 12: 

- Message 12: 
    - no marker !
    - PastMarker.BranchID = (1,3), (3,3)
    - Payload.BranchID = Main
    - MessageMetadata.BranchID = A+C

![](https://imgur.com/yTk30Zq.png)

### Transaction 13 (create Marker): 

- Message 13: 
    - PastMarker.BranchID = A+C+E
    - Payload.BranchID = Main

![](https://imgur.com/eZ6DPr4.png)

### Transaction 14: 

- Message 14: 
    - no marker !
    - PastMarker.BranchID = (1,3), (3,3)
    - Payload.BranchID = Main
    - MessageMetadata.BranchID = A+C

![](https://imgur.com/W8p0Ahw.png)

### Transaction 15 (create Marker): 

- Message 15: 
    - PastMarker.BranchID = [A+C], [.] indicates inheritance
    - Payload.BranchID = Main

![](https://imgur.com/RBbzhwp.png)

### Transaction 16 (create Marker): 

- Message 16: 
    - PastMarker.BranchID = [A+C], [.] indicates inheritance
    - Payload.BranchID = Main

![](https://imgur.com/K6BbedV.png)

### Transaction 17 (create Marker, double spend): 

- Message 17: 
    - double spends message 6
    - create Branch G
    - PastMarker.BranchID = G
    - Payload.BranchID = G
- Message 6:
    - create Branch F
    - change PastMarker.BranchID = F
- Message 8: 
    - change PastMarker.BranchID = F
- Message 11:
    - change PastMarker.BranchID = F+C
- Message 12:
    - change MessageMetadata.BranchID = F+C
- Message 14:
    - change MessageMetadata.BranchID = F+C
- Message 15:
    - change PastMarker.BranchID = [F+C]
- Message 16:
    - change PastMarker.BranchID = [F+C]

![](https://imgur.com/yweNyhP.png)

### Transaction 18 (create Marker, double spend): 

- Message 18:
    - double spend message 14
    - create Branch I
    - PastMarker.BranchID = I+C
    - Payload.BranchID = I
- Message 14: 
    - create Branch H
    - change payload.BranchID = H
    - change MesssageMetadata.BranchID = H+C
- Message 15:
    - change PastMarker.BranchID = [H+C],
    - change MesssageMetadata.BranchID =H+C
- Message 16:
    - change PastMarker.BranchID = [H+C],
    - change MesssageMetadata.BranchID =H+C

![](https://imgur.com/XIUlZ85.png)

### Transaction 19 (create Marker): 

- Message 19:
    - PastMarker.BranchID = [F], [.] indicates indirect mapping of the marker to the Branch
    - Payload.BranchID = Main

![](https://imgur.com/JF3TDo8.png)

### Transaction 20: 

- PastMarker.BranchID = H+C
- Payload.BranchID = Main
- PastMarker(6,5), PastMarker(1,4) = [Branch(H+C)] : PastMarkers are indirectly mapped to Branch(H+C)


![](https://imgur.com/d6NrAzw.png)

