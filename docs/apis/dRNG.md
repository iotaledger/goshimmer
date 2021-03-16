The dRNG module provides API to retrieve both information on the committees as well as the latest randomness.

```bash
curl --request GET \
  --url http://<address>:<port>/drng/info/committee
```

Should give a similar output:

```json
{
  "committees": [
    {
      "instanceID": 1339,
      "threshold": 4,
      "identities": [
        "GUdTwLDb6t6vZ7X5XzEnjFNDEVPteU7tVQ9nzKLfPjdo",
        "68vNzBFE9HpmWLb2x4599AUUQNuimuhwn3XahTZZYUHt",
        "Dc9n3JxYecaX3gpxVnWb4jS3KVz1K1SgSK1KpV1dzqT1",
        "75g6r4tqGZhrgpDYZyZxVje1Qo54ezFYkCw94ELTLhPs",
        "CN1XLXLHT9hv7fy3qNhpgNMD6uoHFkHtaNNKyNVCKybf",
        "7SmttyqrKMkLo5NPYaiFoHs8LE6s7oCoWCQaZhui8m16",
        "CypSmrHpTe3WQmCw54KP91F5gTmrQEL7EmTX38YStFXx"
      ],
      "distributedPK": "901b0def227621364c784124cfa54a1e9b582a3867004511e6810307a8985ef84ff02541a9de4f30b8ff2d0b2972735c"
    },
    {
      "instanceID": 1,
      "threshold": 3,
      "identities": [
        "AheLpbhRs1XZsRF8t8VBwuyQh9mqPHXQvthV5rsHytDG",
        "FZ28bSTidszUBn8TTCAT9X1nVMwFNnoYBmZ1xfafez2z",
        "GT3UxryW4rA9RN9ojnMGmZgE2wP7psagQxgVdA4B9L1P",
        "4pB5boPvvk2o5MbMySDhqsmC2CtUdXyotPPEpb7YQPD7",
        "64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga"
      ],
      "distributedPK": "884bc65f1d023d84e2bd2e794320dc29600290ca7c83fefb2455dae2a07f2ae4f969f39de6b67b8005e3a328bb0196de"
    }
  ]
}
```


```bash
curl --request GET \
  --url http://<address>:<port>/drng/info/randomness
```

Should give a similar output:

```json
{
  "randomness": [
    {
      "instanceID": 1,
      "round": 489295,
      "timestamp": "2020-10-08T09:40:30.291940965Z",
      "randomness": "Dh62wImUx3zQ7sjZ6ulje+NvvPY1DYaUFrTmCP7gOWLQIHHcAF5o9bvRy0tanoHb3q3OlNHKO/DmpDc+SB6A1g=="
    },
    {
      "instanceID": 1339,
      "round": 229624,
      "timestamp": "2020-10-08T09:40:30.253587073Z",
      "randomness": "EfQLVEwGlKrHhPAuZPd+JYXl19ZH03MW9m+D07UHjqnO/AuthbCYFY8AsVe4wu0s3P63HQ5N0dv5X+N5kvrIWw=="
    }
  ]
}
```