# Marker tool test


The Marker tool is tested for the booker with the following scenario:

![](https://imgur.com/sCkXXrG.png)

## The test
The transaction-by-transaction development of the data associated with the marker DAG is as follows:

### Transaction 1 (Marker): 

- Tx 1: 
    - create sequence ID 1, for now sequence ID of Master = 1
    - PastMarker.BranchID = MasterBranchID
    - Payload.BranchID = MasterBranchID

Note that MasterBranchID does not own sequence ID 1, although the PastMarker has sequence ID 1.

![](https://imgur.com/Wsobgv0.png)

### Transaction 2 (Marker, double spend): 

- Tx 2 double spends tx 1
- Tx 1:
    - create Branch A
    - Branch A gets sequence ID = 1
    - PastMarker.BranchID = A
    - Payload.BranchID = A
- Tx 2: 
    - create Branch B
    - create sequence ID 2
    - PastMarker.BranchID = B
    - Payload.BranchID = B

![](https://imgur.com/9SqyjlY.png)

### Transaction 3 (Marker, double spend): 

- Tx 3 double spends tx 1
- Tx 3 
    - create Branch C
    - create sequence ID 3
    - PastMarker.BranchID = C
    - Payload.BranchID = C

![](https://imgur.com/N8xffwD.png)

### Transaction 4 (Marker): 

- Tx 4
    - create sequence ID 4, for now sequence ID of Master = 4
    - PastMarker.BranchID = Master
    - Payload.BranchID = Master

![](https://imgur.com/GIJjo0b.png)

### Transaction 5 (Marker): 

- Tx 5
    - PastMarker.BranchID = Master
    - Payload.BranchID = Master

![](https://imgur.com/YMGAbiF.png)

### Transaction 6 (Marker): 

- Tx 6
    - PastMarker.BranchID = A
    - Payload.BranchID = Master, since no double spend

![](https://imgur.com/ZtSYkvq.png)

### Transaction 7 (Marker): 

- Tx 7
    - PastMarker.BranchID = C
    - Payload.BranchID = Master, since no double spend

![](https://imgur.com/eqmTCnY.png)

### Transaction 8 (Marker): 

- Tx 8
    - PastMarker.BranchID = A
    - Payload.BranchID = Master, since no double spend

![](https://imgur.com/gbLlGgd.png)

### Transaction 9 (Marker, double spend): 

- Tx 9
    - PastMarker.BranchID = C
    - Payload.BranchID = Master, since no double spend

![](https://imgur.com/i6ZFvnF.png)

### Transaction 10 (Marker, double spend): 

- Tx 5
    - create Branch D
    - change: PastMarker.BranchID = D
    - change: Payload.BranchID = D
- Tx 10
    - double spends tx 5
    - create Branch E
    - change: PastMarker.BranchID = D

## TODO : why does creation of Branch D not create a sequence 5

![](https://imgur.com/lImMAQY.png)

### Transaction 11: 

![](https://imgur.com/WT0qEEH.png)

### Transaction 12: 

![](https://imgur.com/yTk30Zq.png)

### Transaction 13: 

![](https://imgur.com/eZ6DPr4.png)

### Transaction 14: 

![](https://imgur.com/W8p0Ahw.png)

### Transaction 15: 

![](https://imgur.com/RBbzhwp.png)

### Transaction 16: 

![](https://imgur.com/K6BbedV.png)

### Transaction 17: 

![](https://imgur.com/yweNyhP.png)

### Transaction 18: 

![](https://imgur.com/XIUlZ85.png)

### Transaction 19: 

![](https://imgur.com/JF3TDo8.png)

### Transaction 20: 

![](https://imgur.com/d6NrAzw.png)

