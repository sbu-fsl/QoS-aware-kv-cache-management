# Benchmark result

Environment setup:

```yaml
# config.yaml
inference_engine:
  block_size: 16 # tokens per block
  compute_speed: 50.0 # blocks per second (simulated compute for a missing block)

cache_engine:
  tiers:
    - name: VRAM
      restore_speed: 100.0 # blocks per second
      capacity: 100 # max blocks

    - name: DRAM
      restore_speed: 20.0 # blocks per second
      capacity: 3000 # max blocks

    - name: Disk
      restore_speed: 5.0 # blocks per second
      capacity: 100000 # max blocks

# cmd.yaml
operations:
  - op: store
    blocks: "1-100000"

  - op: restore
    blocks: "99900-100000"

  - op: restore
    blocks: "99450-100000"

  - op: restore
    blocks: "99000-100000"

  - op: restore
    blocks: "90000-100000"

  - op: restore
    blocks: "9000-100000"
```

Results:

| Policy          | OP  | Recompute (#) | VRAM (#) | DRAM (#) | Disk (#) | Latency (ms) |
| --------------- | :-: | :-----------: | :------: | :------: | :------: | :----------: |
| Default         |  1  |       0       |   100    |     1    |    0     |    1000      |
| QoS-no-limit    |  1  |      29       |    61    |    11    |    0     |     610      |
| QoS-6000-cap    |  1  |      29       |    61    |    11    |    0     |     610      |
| Default         |  2  |       0       |   100    |   451    |    0     |   22550      |
| QoS-no-limit    |  2  |     322       |   100    |   129    |    0     |    6450      |
| QoS-6000-cap    |  2  |     322       |   100    |   129    |    0     |    6450      |
| Default         |  3  |       0       |   100    |   901    |    0     |   45050      |
| QoS-no-limit    |  3  |     643       |   100    |   258    |    0     |   12900      |
| QoS-6000-cap    |  3  |     643       |   100    |   258    |    0     |   12900      |
| Default         |  4  |       0       |   100    |   2900   |   7001   |   1400200    |
| QoS-no-limit    |  4  |     6600      |   100    |   2900   |   401    |    145000    |
| QoS-6000-cap    |  4  |     6000      |   100    |   2900   |  1001    |    200200    |
| Default         |  5  |       0       |   100    |   2900   |  88001   |   17600200   |
| QoS-no-limit    |  5  |    79910      |   100    |   2900   |   8091   |   1618200    |
| QoS-6000-cap    |  5  |     6000      |   100    |   2900   |  82001   |   16400200   |

## Improvements

Improvement is measured as latency reduction versus the `Default` policy for the same restore index. Token distribution shows the share of restored tokens served by recomputation or each tier.

| Policy         | OP  | Latency Improvement | Recompute (%) | VRAM (%) | DRAM (%) | Disk (%) |
| -------------- | :-: | :-----------------: | :-----------: | :------: | :------: | :------: |
| Default        |  1  |       0.00%         |    0.00%      | 99.01%   |  0.99%   |  0.00%   |
| QoS-no-limit   |  1  |      39.00%         |   28.71%      | 60.40%   | 10.89%   |  0.00%   |
| QoS-6000-cap   |  1  |      39.00%         |   28.71%      | 60.40%   | 10.89%   |  0.00%   |
| Default        |  2  |       0.00%         |    0.00%      | 18.15%   | 81.85%   |  0.00%   |
| QoS-no-limit   |  2  |      71.40%         |   58.44%      | 18.15%   | 23.41%   |  0.00%   |
| QoS-6000-cap   |  2  |      71.40%         |   58.44%      | 18.15%   | 23.41%   |  0.00%   |
| Default        |  3  |       0.00%         |    0.00%      |  9.99%   | 90.01%   |  0.00%   |
| QoS-no-limit   |  3  |      71.37%         |   64.24%      |  9.99%   | 25.77%   |  0.00%   |
| QoS-6000-cap   |  3  |      71.37%         |   64.24%      |  9.99%   | 25.77%   |  0.00%   |
| Default        |  4  |       0.00%         |    0.00%      |  1.00%   | 29.00%   | 70.00%   |
| QoS-no-limit   |  4  |      89.64%         |   65.99%      |  1.00%   | 29.00%   |  4.01%   |
| QoS-6000-cap   |  4  |      85.71%         |   59.99%      |  1.00%   | 29.00%   | 10.01%   |
| Default        |  5  |       0.00%         |    0.00%      |  0.11%   |  3.19%   | 96.70%   |
| QoS-no-limit   |  5  |      90.80%         |   87.88%      |  0.11%   |  3.19%   |  8.89%   |
| QoS-6000-cap   |  5  |       6.82%         |    6.59%      |  0.11%   |  3.19%   | 90.11%   |
