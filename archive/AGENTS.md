# AGENTS.md

## Philosophy
Agents are assistants that:
- analyze
- propose
- evaluate
- monitor

They DO NOT:
- own data
- execute irreversible actions
- bypass business rules

## Agent Types

### 1. Market Research Agent
- Inputs: product, category
- Outputs: demand, competition, trends

### 2. Profitability Agent
- Inputs: ASIN, supplier cost
- Outputs: ROI, margin, break-even

### 3. Risk Agent
- Inputs: ASIN, brand
- Outputs: gating, IP risk, listing quality

### 4. Supplier Agent
- Inputs: product/brand
- Outputs: supplier candidates, ranking

### 5. Outreach Agent
- Inputs: supplier context
- Outputs: personalized outreach messages

### 6. Replenishment Agent
- Inputs: inventory + velocity
- Outputs: reorder recommendations

### 7. Experiment Agent (Autoresearch)
- Proposes improvements
- Runs controlled experiments
- Evaluates outcomes

## Agent Execution Model
- All agents run via AgentRuntime interface
- Agents must return structured outputs
- All outputs must be explainable

## Safety Rules
- No agent can:
  - send real emails without approval
  - place orders
  - change pricing automatically (unless flagged safe)

## Memory
- Agents can read:
  - past experiments
  - deal outcomes
- Agents cannot mutate core data directly