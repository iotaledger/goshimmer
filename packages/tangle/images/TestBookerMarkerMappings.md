# Test of Booker and Marker Mapping


The Marker tool is tested for the booker with the following scenario:

![](https://imgur.com/sCkXXrG.png)


## General rules

We apply the following defintions:
- a colored field indicates that a value is set
- a field without color indicates the values are inherited
- [ . ] for a BranchID indicates inheritance. For a data message or transaction that inherits a branchID there is no action/update of mapping  necessary.
- Data messages always have Payload.BranchID = Main

The transaction-by-transaction development of the data associated with the marker DAG is as follows:

# 
## Step-by-step analysis
### Transaction 1 (create Marker): 

- Transaction 1: 
    - create sequence ID 1, for now sequence ID of Main = 1
    - PastMarker.BranchID = MainBranchID
    - Payload.BranchID = MainBranchID

Note that MainBranchID does not own sequence ID 1, although the PastMarker has sequence ID 1.

![](https://imgur.com/Wsobgv0.png)

### Transaction 2 (create Marker, double spend): 

- Transacion 2 double spends transaction 1
- Transaction 1:
    - create Branch A
    - Branch A gets sequence ID = 1
    - PastMarker.BranchID = A
    - Payload.BranchID = A
- Transaction 2: 
    - create Branch B
    - create sequence ID 2
    - PastMarker.BranchID = B
    - Payload.BranchID = B

![](https://imgur.com/9SqyjlY.png)

### Transaction 3 (create Marker, double spend): 

- Transaction 3 double spends transaction 1
- Transaction 3 
    - create Branch C
    - create sequence ID 3
    - PastMarker.BranchID = C
    - Payload.BranchID = C

![](https://imgur.com/N8xffwD.png)

### Data message 4 (create Marker): 

- Data message 4
    - create sequence ID 4, for now sequence ID of Main = 4
    - PastMarker.BranchID = Main
    - Payload.BranchID = Main (since data)

![](https://imgur.com/GIJjo0b.png)

### Transaction 5 (create Marker): 

- Transaction 5
    - PastMarker.BranchID = Main
    - Payload.BranchID = Main

![](https://imgur.com/YMGAbiF.png)

### Transaction 6 (create Marker): 

- Transaction 6
    - PastMarker.BranchID = [A]
    - Payload.BranchID = [A] (note: incorrectly shown in the Figure)

![](https://imgur.com/ZtSYkvq.png)

### Data message 7 (create Marker): 

- Data message 7
    - PastMarker.BranchID = [C]
    - Payload.BranchID = Main, (since data)

![](https://imgur.com/eqmTCnY.png)

### Data message 8 (create Marker): 

- Data message 8
    - PastMarker.BranchID = [A] 
    - Payload.BranchID = Main, (since data)

![](https://imgur.com/gbLlGgd.png)

### Data message 9 (create Marker): 

- Data message 9
    - PastMarker.BranchID = [C]
    - Payload.BranchID = Main (since data)

![](https://imgur.com/i6ZFvnF.png)

### Transaction 10 (create Marker, double spend): 

- Transaction 5
    - create Branch D
    - change: PastMarker.BranchID = D
    - change: Payload.BranchID = D
- Transaction 10
    - double spends message 5
    - create Branch E
    - PastMarker.BranchID = B+E
    - Payload.BranchID = E

![](https://imgur.com/lImMAQY.png)

### Data message 11 (create Marker): 

- Data message 11: 
    - PastMarker.BranchID = A+C
    - Payload.BranchID = Main (since data)

![](https://imgur.com/WT0qEEH.png)

### Data message 12: 

- Data message 12: 
    - no marker !
    - PastMarker.BranchID = (1,3), (3,3)
    - Payload.BranchID = Main (since data)
    - MessageMetadata.BranchID = A+C (Message needs to be individually mapped to branches)

![](https://imgur.com/yTk30Zq.png)

### Data message 13 (create Marker): 

- Data message 13: 
    - PastMarker.BranchID = A+C+E
    - Payload.BranchID = Main (since data)

![](https://imgur.com/eZ6DPr4.png)

### Transaction 14: 

- Transaction 14: 
    - no marker !
    - PastMarker.BranchID = (1,3), (3,3)
    - Payload.BranchID = A, (note: incorrectly shown in the Figure)
    - MessageMetadata.BranchID = A+C (Message needs to be individually mapped to branches)

![](https://imgur.com/W8p0Ahw.png)

### Data message 15 (create Marker): 

- Data message 15: 
    - PastMarker.BranchID = [A+C]
    - Payload.BranchID = Main (since data)

![](https://imgur.com/RBbzhwp.png)

### Data message 16 (create Marker): 

- Data message 16: 
    - PastMarker.BranchID = [A+C]
    - Payload.BranchID = Main (since data)

![](https://imgur.com/K6BbedV.png)

### Transaction 17 (create Marker, double spend): 

- Transaction 17: 
    - double spends transaction 6
    - create Branch G
    - PastMarker.BranchID = G
    - Payload.BranchID = G
- Transaction 6:
    - create Branch F
    - change PastMarker.BranchID = F
- Data message 8: 
    - change PastMarker.BranchID = [F] 
- Data message 11:
    - change PastMarker.BranchID = F+C 
- Data message 12:
    - change MessageMetadata.BranchID = F+C (Message needs to be individually mapped to branches)
- Transaction 14:
    - change Payload.BranchID = F, (note: incorrectly shown in the Figure)
    - change MessageMetadata.BranchID = F+C (Message needs to be individually mapped to branches)
- Data message 15:
    - change PastMarker.BranchID = [F+C]  
- Data message 16:
    - change PastMarker.BranchID = [F+C]  

![](https://imgur.com/yweNyhP.png)

### Transaction 18 (create Marker, double spend): 

- Transaction 18:
    - double spend transaction 14
    - create Branch I, because it is a more specialized version of F due to the double spend.
    - PastMarker.BranchID = I+C
    - Payload.BranchID = I
- Transaction 14: 
    - create Branch H
    - change payload.BranchID = H
    - change MesssageMetadata.BranchID = H+C (Message needs to be individually mapped to branches)
- Data message 15:
    - change PastMarker.BranchID = [H+C],
    - change MessageMetadata.BranchID =H+C (Message needs to be individually mapped to branches)
- Data message 16:
    - change PastMarker.BranchID = [H+C],
    - change MesssageMetadata.BranchID =H+C (Message needs to be individually mapped to branches)

![](https://imgur.com/XIUlZ85.png)

### Data  message 19 (create Marker): 

- Data message 19:
    - PastMarker.BranchID = [F], [.] indicates indirect mapping of the marker to the Branch
    - Payload.BranchID = Main (since data)

![](https://imgur.com/JF3TDo8.png)

### Data message 20: 

- Data message 20:
    - PastMarker.BranchID = H+C
    - Payload.BranchID = Main (since data)
    - PastMarker(6,5), PastMarker(1,4) = [Branch(H+C)] : PastMarkers are indirectly mapped to Branch(H+C)


![](https://imgur.com/d6NrAzw.png)

