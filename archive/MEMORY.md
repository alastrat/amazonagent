# MEMORY.md

## Purpose
Define what the system remembers and how it learns over time.

## Memory Types

### 1. Deal Performance Memory
- predicted vs actual margin
- sales velocity accuracy
- buy box win rate

### 2. Supplier Memory
- response rate
- approval rate
- pricing competitiveness
- reliability

### 3. Experiment Memory
- experiment name
- variants
- winner
- confidence
- impact

### 4. Agent Performance Memory
- accuracy of predictions
- false positives
- false negatives

## Usage
- Agents can read memory to improve decisions
- Autoresearch engine uses memory to propose new experiments

## Constraints
- Memory is append-only where possible
- No silent overwrites
- All changes are auditable