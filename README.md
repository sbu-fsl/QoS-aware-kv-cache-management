# QoS-aware KV Cache Management

A simulated heterogeneous multi-tier KV cache system with a CLI-based inference engine that stores, looks up, and restores blocks across storage tiers.
This project aims to replicate the real-world distribution of KV caches across storage tiers during LLM inference.
The project supports applying QoS-aware cache management policies with two SLOs. The default SLO targets lower TTFT, resulting in reduced queue delays and lower end-to-end latency. However, by setting a cap on computed blocks, an energy-efficiency-oriented SLO can also be enforced.

## Simulator Design

### Block

The unit of storage. Each block holds a fixed number of tokens (`block_size`). Blocks are identified by integer IDs.

### Tiers

Storage is organized into ordered tiers. The order determines restore priority. The first tier in the list is tried first (fastest). Each tier has a speed (blocks/second) and a capacity (max blocks). When a tier is full, the least-recently-used block is evicted (LRU policy).

Default tiers:

| Tier | Restore Speed | Capacity |
| ---- | ------------- | -------- |
| VRAM | 100 blk/s     | 10       |
| DRAM | 20 blk/s      | 30       |
| Disk | 5 blk/s       | 100      |

### Inference Engine

The CLI interface that drives store, lookup, and restore operations and reports what happened. It also falls back to a recomputation logic, which stores the blocks that are missing for a computation cost.

### Cache Engine

Bookkeeps blocks across all tiers. On store, a block is written to VRAM and DRAM. If VRAM is full, eviction will happen. If DRAM is full, offloading to other tiers will happen.

On restore, each block is served from the first (fastest) tier that has it. Tiers operate in parallel, so overall restore time is the maximum individual tier restore time and recomputation time.

### Storage System

Each tier stores blocks and evicts LRU blocks when capacity is exceeded. Evictions are reported back to the user.

## Configuration

Tunings are read from `config.yaml` at startup. If the file is not found, built-in defaults are used.

```yaml
inference_engine:
  block_size: 16 # tokens per block
  compute_speed: 50.0 # blocks per second (simulated compute for a missing block)

cache_engine:
  in_memory_run: true # if true, operates in-memory without file persistence (for less overhead in tests)
  restore_mode: "qos" # greedy or qos
  max_recompute_blocks: -1 # cap on how many cached blocks may be force-recomputed; -1 for no cap
  tiers:
    - name: VRAM
      restore_speed: 100.0 # blocks per second
      capacity: 10 # max blocks

    - name: DRAM
      restore_speed: 20.0 # blocks per second
      capacity: 30 # max blocks

    - name: Disk
      restore_speed: 5.0 # blocks per second
      capacity: 100 # max blocks
```

> Tier cache state is persisted as JSON files in the `data/` directory (one file per tier), so the cache survives restarts.

## Building

```sh
make build        # compile both binaries into bin/
make run          # interactive CLI only
make autorun      # batch runner only
```

## Running

### Interactive CLI

```sh
make run

# or directly:
go run main.go qos
```

Flags:

| Flag       | Default       | Description                    |
| ---------- | ------------- | ------------------------------ |
| `--config` | `config.yaml` | Path to config file            |
| `--data`   | `data/`       | Directory for tier cache files |

### Batch Auto-Run

Execute a sequence of operations defined in a YAML file, write the full report to a file, and print per-operation speed to the console.

```sh
make autorun

# or with overrides:
make autorun CMD_YAML=my.yaml OUT=report.txt
```

Flags:

| Flag        | Default       | Description                    |
| ----------- | ------------- | ------------------------------ |
| `---config` | `config.yaml` | Path to config file            |
| `--data`    | `data/`       | Directory for tier cache files |
| `--cmd`     | `cmd.yaml`    | Path to operations file        |
| `--out`     | `out.txt`     | Path to write the full report  |

#### cmd.yaml format

Each entry has an `op` (`store` or `restore`) and a `blocks` field. Blocks can be specified as a comma-separated list or a range:

```yaml
operations:
  - op: store
    blocks: "1-10" # range: blocks 1 through 10

  - op: store
    blocks: "20, 30, 40" # comma-separated list

  - op: restore
    blocks: "1-5"

  - op: restore
    blocks: "5, 9, 20, 99"
```

#### Console output (speed only)

```text
[Op 1] store [1 2 3 4 5 6 7 8 9 10] — 10 block(s) stored
[Op 2] store [20 30 40] — 3 block(s) stored
[Op 3] restore [1 2 3 4 5] — 100.00 blocks/s  (overall: 50.00ms)
[Op 4] restore [5 9 20 99] — 133.33 blocks/s  (overall: 30.00ms)

Full report written to out.txt
```

The full structured report (per-block source, tier lanes, evictions) is written exclusively to `out.txt`.

## Other Make Targets

```sh
make test    # go test ./...
make vet     # go vet ./...
make fmt     # go fmt ./...
make clean   # remove bin/ and out.txt
make help    # list all targets
```

## CLI Commands

```text
store   <id,...>   Store blocks across all tiers
lookup  <id,...>   Check which tiers have each block
restore <id,...>   Restore blocks; compute+auto-store if missing
status             Show tier fill levels
help               Show command list
exit               Quit
```

## Example Session

```text
> store 1,2,3,4,5,6,7,8,9
STORE — 9 block(s)
Block       Evictions
--------------------------------------------------
  #1         none
  #2         none
  ...

> lookup 1,2,3
LOOKUP — 3 block(s)  [hits: 3  misses: 0]
Block       Hit       Locations
--------------------------------------------------
  #1         HIT       GPU_VRAM, CPU_DRAM, SSD_Disk
  #2         HIT       GPU_VRAM, CPU_DRAM, SSD_Disk
  #3         HIT       GPU_VRAM, CPU_DRAM, SSD_Disk

> lookup 7,10,11
LOOKUP — 3 block(s)  [hits: 1  misses: 2]
Block       Hit       Locations
--------------------------------------------------
  #7         HIT       GPU_VRAM, CPU_DRAM, SSD_Disk
  #10        miss      —
  #11        miss      —

> restore 1,2,3,7,10
RESTORE — 5 block(s)  [from cache: 4  computed: 1]
Block       Source        Time        Note
------------------------------------------------------------
  #1         GPU_VRAM      10.00ms
  #2         GPU_VRAM      10.00ms
  #3         GPU_VRAM      10.00ms
  #7         GPU_VRAM      10.00ms
  #10        COMPUTE       500.00ms    auto-stored for next time
------------------------------------------------------------
  Overall (parallel) restore time: 500.00ms

> status
STATUS
Tier            Used/Cap    Blocks
------------------------------------------------------------
  GPU_VRAM      10/10        #1 #2 #3 #4 #5 #6 #7 #8 #9 #10
  CPU_DRAM      10/30        #1 #2 #3 #4 #5 #6 #7 #8 #9 #10
  SSD_Disk      10/100       #1 #2 #3 #4 #5 #6 #7 #8 #9 #10
```

### Restore logic

- Each block is served from the **fastest tier** that has it (tiers are checked in config order).
- Blocks found in different tiers are restored **in parallel** — overall time is the slowest individual lane.
- A block not found in any tier is **computed** at `compute_speed` (blocks/second) and **automatically stored** across all tiers for next time.

### Eviction

When a tier reaches capacity, the **least-recently-used** block is evicted to make room. Evictions are reported per-tier in the store output.
