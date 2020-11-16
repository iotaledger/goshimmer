# Approval switch
The Tangle builds approval of a given message by directly or indirectly attaching other messages in its future cone. Due to different reasons, such as the TSA not picking up a given message before its timestamp is still *fresh* or because its past cone has been rejected, a message can become orphan. This implies that the message cannot be included in the Tangle history since all the recent tips do not contain it in their past cone and thus, it cannot be retrieved during solidification. As a result, it might happen that honest messages and transactions would need to be reissued or reattached.
To overcome this limitation, we propose the `approval switch`. The idea is to minimize honest messages along with transactions getting orphaned, by assigning a different meaning to the parents of a message.

## Detailed design
Each message can express two levels of approval with respect to its parents:
* **Strong**: it defines approval for both the referenced message along with its entire past cone.
* **Weak**: it defines approval for the referenced message but not for its past cone.

Let's consider the following example:

![](https://i.imgur.com/rfpnkcg.png)

Message *D* contains a transaction that has been rejected, thus, due to the monotonicity rule, its future cone must be orphaned. Both messages *F* (transaction) and *E* (data) directly reference *D* and, traditionally, they should not be considered for tip selection. However, by intorducing the approval switch, these messages can be picked up via a **weak** reference as messages *G* and *H* show.

We define two categories of eligible messages:
* **Strong message**: 
    * It is eligible
    * Its payload is liked with level of knowledge >=2
    * Its branch is **liked** with level of knowledge >= 2
* **Weak message**: 
    * It is eligible
    * Its payload is liked with level of knowledge >=2
    * Its branch is **not liked** with level of knowledge >= 2
    
We call *strong approver of x* (or *strong child of x*) any strong message *y* approving *x* via a strong reference. Similarly, we call *weak approver of x* (or *weak child of x*) any strong message *y* approving *x* via a weak reference.

### TSA
We define two separate tip types:
* **Strong tip**: 
    * It is a strong message
    * It is not directly referenced by any strong message via strong parent
* **Weak tip**: 
    * It is a weak message
    * It is not directly referenced by any strong message via weak parent

Consequently, a node keeps track of the tips by using two distincts tips sets:
* **Strong tips set**: contains the strong tips
* **Weak tips set**: contains the weak tips

Tips of both sets must be managed according to the local perception of the node. Hence, a strong tip loses its tip status if it gets referenced (via strong parent) by a strong message. Similarly, a weak tip loses its tip status if it gets referenced (via weak parent) by a strong message. This means that weak messages approving via either strong or weak parents, do not have an impact on the tip status of the messages they reference. 

The Tip Selection Algorithm (TSA) returns a set of tips. It takes an integer *n*, that is the desired number tips to be returned. 
1. select **1** **strong tip** from the strong tips set; 
2. Cycling in a loop with *n-1* steps, if the weak tips set is **not empty**, select, with a given probability *p*, a weak tip from the weak tips set, or with probability *1-p*, a strong tip from the strong tips set;

### Branch management
A message inherits the branch of its strong parents, while it does not inherit the branch of its weak parents.

### Approval weight
The approval weight of a given message takes into account all of its future cone built over all its strong approvers. 
Let's consider the following example: 

![](https://i.imgur.com/a9FTyyg.png)

*E* is a weak message strongly approving *B* and *D*. When considering the approval weight of *B*, only the strong approvers of its future cone are used, thus, *D, E, F*. Note that, the approval weight of $E$ would instead be built over *G, H, I*. Therefore, its approval weight does not add up to its own weight (for instance, when looking at the approval weight of *B*). 

### Solidification
The solidification process does not change, both parent types are used to progress.

### Test cases
* message *x* strongly approves a strong message *y*: ok
* message *x* weakly approves a strong message *y*: it's weird, counts for approval weight of *y* but does not affect the tip status of *y*
* message *x* strongly approves a weak message *y*: *x* becomes a weak message
* message *x* weakly approves a weak message *y*: ok