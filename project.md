# Home Network Observatory

## Vision

Home networks are becoming increasingly complex.

Modern homes often have multiple access points, mesh systems, smart devices, streaming services, video calls, gaming consoles, and dozens of IoT devices competing for bandwidth. When something goes wrong, the experience is almost always the same:

> "The Wi-Fi feels slow."

Unfortunately, that observation provides almost no information about **where** the problem actually exists.

Is the issue:

* The wireless signal?
* The access point?
* The router?
* DNS?
* The ISP?
* A congested device?
* A bad roaming decision?
* Temporary interference?

Today, troubleshooting is largely based on guesswork.

The goal of this project is to replace guessing with **measurement**.

---

# Project Goal

Build a distributed home network observability platform capable of continuously monitoring, visualizing, and diagnosing the health of a home network.

The system should answer questions such as:

* Is the problem Wi-Fi or the ISP?
* Which room experiences degraded connectivity?
* When did the problem begin?
* How often does it happen?
* Which access point is each device connected to?
* Has performance changed over the last week?
* What changed immediately before an outage?
* What is the most likely root cause?

Ultimately, the system should function similarly to enterprise observability platforms—but tailored specifically to residential networking.

---

# Guiding Principles

## Measure everything

Never assume.

Collect measurements that allow hypotheses to be confirmed or disproven.

---

## Build incrementally

Every milestone should produce a useful system.

The project should never require months of work before becoming valuable.

---

## Solve real problems

Every new feature should improve the ability to understand or diagnose actual network behavior.

Avoid building features simply because they are technically interesting.

---

## Separate data collection from analysis

Probes should collect measurements.

The central server should analyze them.

This separation keeps the architecture flexible and allows new probe types to be added without changing the backend.

---

## Hardware should support the software—not drive it

The objective is to build an observability platform.

Learning embedded systems and electronics is a welcome side effect, not the primary objective.

---

# High-Level Architecture

```
                    Internet
                        │
                 Home Router / AP
                        │
          ┌─────────────┴─────────────┐
          │                           │
      Wired Mini PC              Wi-Fi Network
          │                           │
     Collector/API          Distributed Probes
          │             (Raspberry Pi initially)
          │
     Time-Series Storage
          │
     Dashboard & Analytics
          │
      Diagnosis Engine
```

---

# System Components

## Central Server

Runs on an always-on mini PC.

Responsibilities include:

* Receiving measurements
* Storing historical data
* Managing probes
* Visualization
* Alerting
* Incident detection
* Future firmware/configuration management

The server represents the **control plane** of the system.

---

## Probe Agents

Probes are lightweight monitoring agents deployed throughout the home.

Each probe continuously measures its local network environment.

Examples include:

* Office
* Upstairs
* Basement
* Living room
* Garage

Each probe periodically reports measurements to the central server.

Initially these probes will run on Raspberry Pis.

Future versions may support:

* ESP32 devices
* Desktop computers
* Laptops
* Virtual machines
* Additional embedded hardware

The backend should treat every probe identically regardless of hardware.

---

## Time-Series Storage

Measurements are stored historically.

This enables:

* Trend analysis
* Historical comparisons
* Regression detection
* Incident review
* Long-term reporting

---

## Dashboard

The dashboard should answer three questions immediately:

1. Is the network healthy?
2. Where is the problem?
3. Why do we believe that?

Charts are valuable, but the goal is understanding rather than visualization alone.

---

## Diagnosis Engine

The diagnosis engine is the heart of the project.

Instead of only displaying graphs, the system should explain them.

Example:

> Upstairs Wi-Fi degraded for 8 minutes.
>
> Evidence:
>
> * RSSI decreased 14 dB
> * Packet loss increased to 18%
> * Office probe remained healthy
> * Router latency remained normal
>
> Likely cause:
>
> Local wireless interference affecting the upstairs access point.

The long-term objective is to build a system that assists in troubleshooting rather than merely displaying metrics.

---

# Roadmap

## Phase 1 — Foundation

Objective:

Create a working observability platform with a single probe.

Features:

* Collector service
* Probe agent
* Historical storage
* Dashboard
* Ping measurements
* Packet loss
* Jitter
* Internet latency

Success criteria:

"I can see historical network health."

---

## Phase 2 — Distributed Probes

Objective:

Deploy multiple Raspberry Pi probes.

New capabilities:

* Per-room monitoring
* Signal strength
* Access point identification
* Local latency
* Comparative analysis

Success criteria:

"I know which areas of the house experience problems."

---

## Phase 3 — Network Diagnostics

Add deeper measurements:

* DNS performance
* Throughput testing
* Gateway latency
* Local network latency
* Connection events
* Interface statistics

Success criteria:

"I can distinguish Wi-Fi problems from ISP problems."

---

## Phase 4 — Incident Detection

Automatically identify network events.

Examples:

* Packet loss spike
* Frequent roaming
* AP instability
* Internet outage
* High latency
* Throughput degradation

The system should detect incidents automatically rather than relying solely on visual inspection.

---

## Phase 5 — Root Cause Analysis

Build a diagnosis engine that correlates multiple measurements into likely explanations.

Instead of presenting metrics, present conclusions.

---

## Phase 6 — Hardware Expansion

Introduce custom hardware where it adds value.

Possible additions include:

* ESP32 probes
* Custom PCBs
* Battery-powered monitoring nodes
* Environmental sensors
* Power monitoring
* LED status indicators

The protocol between probes and the central server should remain unchanged.

---

## Phase 7 — Home Observability Platform

Expand beyond networking.

Potential domains include:

* Server health
* UPS monitoring
* Storage monitoring
* Environmental sensors
* Power consumption
* Home automation
* Security events

At this point the project evolves from a network monitor into a complete home observability platform.

---

# Technology Strategy

Initial implementation:

* Go for probe agents
* Go or Rails for the collector
* Prometheus for metrics
* Grafana for dashboards
* Raspberry Pi probes
* Docker-based deployment

Potential future technologies:

* MQTT
* PostgreSQL / TimescaleDB
* Rails-based management UI
* ESP32 firmware
* Custom PCB design
* OTA firmware updates

Technology choices may evolve, but the architectural principles should remain stable.

---

# Long-Term Vision

The goal is not simply to build another dashboard.

The goal is to create a system that continuously observes the home network, explains what it sees, and helps identify the root cause of problems before they become frustrating.

Along the way, the project serves as a practical vehicle for learning:

* Networking
* Distributed systems
* Observability
* Time-series data
* Linux systems programming
* Go
* Embedded development
* Hardware design

The final result should be something that is both personally useful and technically rewarding—a system that solves real problems while steadily expanding knowledge across the hardware and software stack.

