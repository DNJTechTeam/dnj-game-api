# Proposta de Equação de Pontos — DNJ Game

**Issue:** KAN-13 (DNJ-011)  
**Data:** 2026-09-07  
**Status:** Proposta para aprovação  

---

## 1. Inventário das Fontes Atuais

### 1.1 Fontes de Pontuação Identificadas no Ledger (`point_entries`)

| # | Origem | Motivo | Fonte | Pontos Padrão | Configurável |
|---|--------|--------|-------|---------------|--------------|
| 1 | `activity_run_results` | `activity_run_first` | 1º lugar em run competitiva | 50 pts | Não (fixo) |
| 2 | `activity_run_results` | `activity_run_second` | 2º lugar em run competitiva | 30 pts | Não (fixo) |
| 3 | `activity_run_results` | `activity_run_third` | 3º lugar em run competitiva | 20 pts | Não (fixo) |
| 4 | `activity_run_results` | `activity_run_participation` | Check-in em run (checkpoint/live) | Variável | Sim (`CheckInPoints`) |
| 5 | `schedule_qr_checkin` | `schedule_qr_checkin` | Check-in por QR de programação | Variável | Sim (`CheckInPoints`) |
| 6 | `moment` | `moment_challenge_award` | Moment de desafio aprovado | Variável | Sim (`MomentPoints`) |
| 7 | `moment` | `moment_moderation_reversal` | Reversão admin (negativo) | -Variável | Sim |

### 1.2 Fontes que NÃO Pontuam

| Fonte | Comportamento |
|-------|---------------|
| **Publicação livre (Momentos)** | Vale zero pontos, mesmo com desafio ativo |
| **Posição no ranking** | Derivada dos pontos, não gera pontos |
| **Check-in sem QR** | Não registrado no ledger |
| **Desafio Momento (se não aprovado)** | Não gera pontos até moderação |

### 1.3 Observações sobre Desafio Especial

O **Desafio Especial** não aparece explicitamente como origem separada no ledger. Possíveis cenários:
- Está incluso em `moment_challenge_award` (mesmo mecanismo de Momentos)
- Possui mecanismo próprio não documentado no ledger atual
- Será implementado em iteração futura

**Necessita validação:** Confirmar se Desafio Especial usa `moment_challenge_award` ou possui origem exclusiva.

---

## 2. Análise dos Problemas Atuais

### 2.1 Problemas Identificados

| # | Problema | Impacto | Severidade |
|---|----------|---------|------------|
| 1 | **Valores fixos para 1º/2º/3º lugar** (50/30/20) não consideram número de participantes | Run com 2 participantes recebe mesmos pontos que run com 50 | Alta |
| 2 | **CheckInPoints e MomentPoints configuráveis** sem validação de limites | Admin pode configurar valores absurdos (0 ou 10.000) | Média |
| 3 | **Sem limite diário/semanal** de pontos por fonte | Usuário pode acumular pontos infinitos via QR repeats | Alta |
| 4 | **Desafio Especial** sem definição clara de pontuação | Ambiguidade na regra de negócio | Média |
| 5 | **Reversão de pontos** (moderação) sem limite ou justificativa obrigatória | Risco de manipulação | Média |
| 6 | **Sem progressão de dificuldade** | Atividades fáceis rendem mesmos pontos sempre | Baixa |

### 2.2 Comparação com Boas Práticas

| Prática | Situação Atual | Recomendação |
|---------|----------------|--------------|
| **Pontuação proporcional ao engajamento** | Parcial (ranking usa pontos fixos) | Adicionar multiplicador baseado em participação |
| **Limites diários/semanais** | Não existe | Implementar caps por fonte |
| **Bônus por consistência** | Não existe | Adicionar streak de check-ins |
| **Dificuldade progressiva** | Não existe | Escalonar pontos por nível de atividade |
| **Anti-farm** | Básico (idempotência) | Adicionar rate limiting por usuário |

---

## 3. Proposta de Equação Equilibrada

### 3.1 Estrutura de Pontuação Proposta

#### A. Run Competitiva (Activity Run)

| Posição | Fórmula | Exemplo (10 participantes) | Exemplo (30 participantes) |
|---------|---------|---------------------------|---------------------------|
| **1º lugar** | `base + (participantes × 2)` | 50 + 20 = **70 pts** | 50 + 60 = **110 pts** |
| **2º lugar** | `base × 0.6 + (participantes × 1)` | 30 + 10 = **40 pts** | 30 + 30 = **60 pts** |
| **3º lugar** | `base × 0.4 + (participantes × 0.5)` | 20 + 5 = **25 pts** | 20 + 15 = **35 pts** |
| **Participação** | `CheckInPoints` (configurável) | 10 pts | 10 pts |

**Onde:**
- `base` = valores fixos atuais (50/30/20)
- `participantes` = número total de participantes elegíveis da run

**Justificativa:** Run com mais participantes é mais competitivo e merece mais pontos.

#### B. Check-in por QR de Programação

| Cenário | Fórmula | Exemplo |
|---------|---------|---------|
| **Check-in normal** | `CheckInPoints` | 10 pts |
| **Check-in com streak (3+ dias)** | `CheckInPoints × 1.5` | 15 pts |
| **Check-in diário (máximo 5/dia)** | N/A | Limite atingido |

**Cap diário:** 5 check-ins por programa (evita farm).

#### C. Momento de Desafio (Challenge Moment)

| Cenário | Fórmula | Exemplo |
|---------|---------|---------|
| **Desafio aprovado** | `MomentPoints` | 20 pts |
| **Desafio especial aprovado** | `MomentPoints × 2` | 40 pts |
| **Desafio comunitário (10+ aprovações)** | `MomentPoints × 1.5` | 30 pts |

**Cap diário:** 3 desafios pontuados por usuário.

#### D. Desafio Especial

| Cenário | Fórmula | Exemplo |
|---------|---------|---------|
| **Participação** | `CheckInPoints × 2` | 20 pts |
| **1º lugar** | `50 + (participantes × 3)` | 80 pts (10 part.) |
| **2º lugar** | `30 + (participantes × 2)` | 50 pts (10 part.) |

**Observação:** Desafio especial usa mecânica similar a run competitiva, mas com multiplicador maior.

#### E. Momento Livre

| Cenário | Pontos |
|---------|--------|
| **Publicação livre** | 0 pts (sem alteração) |

---

### 3.2 Tabela Resumo da Proposta

| Fonte | Cenário | Pontos Propostos | Limite |
|-------|---------|------------------|--------|
| **Run Competitiva** | 1º lugar | 50 + (N × 2) | Sem limite |
| | 2º lugar | 30 + (N × 1) | Sem limite |
| | 3º lugar | 20 + (N × 0.5) | Sem limite |
| | Participação | CheckInPoints | 1/run |
| **QR Programação** | Normal | CheckInPoints | 5/dia |
| | Streak 3+ dias | CheckInPoints × 1.5 | 5/dia |
| **Desafio Momento** | Aprovado | MomentPoints | 3/dia |
| | Especial | MomentPoints × 2 | 3/dia |
| | Comunitário | MomentPoints × 1.5 | 3/dia |
| **Desafio Especial** | Participação | CheckInPoints × 2 | 1/evento |
| | 1º lugar | 50 + (N × 3) | 1/evento |
| | 2º lugar | 30 + (N × 2) | 1/evento |
| **Momento Livre** | Qualquer | 0 | N/A |

---

### 3.3 Exemplo Prático: Evento com 30 Participantes

#### Cenário: Usuário "João" participa de 1 dia completo

| Atividade | Pontos | Justificativa |
|-----------|--------|---------------|
| Check-in programação (3x) | 30 pts | 10 × 3 (sem streak) |
| Run competitiva - 1º lugar (30 part.) | 110 pts | 50 + (30 × 2) |
| Desafio Momento aprovado | 20 pts | MomentPoints padrão |
| Desafio Especial - participação | 20 pts | CheckInPoints × 2 |
| **Total do dia** | **180 pts** | |

#### Cenário: Usuário "Maria" participa de 3 dias (streak)

| Dia | Atividade | Pontos |
|-----|-----------|--------|
| **Dia 1** | Check-in (3x) + Run (2º) + Momento | 30 + 40 + 20 = 90 pts |
| **Dia 2** | Check-in (3x streak) + Run (1º) + Momento especial | 45 + 70 + 40 = 155 pts |
| **Dia 3** | Check-in (3x streak) + Desafio Especial (1º) | 45 + 110 = 155 pts |
| **Total 3 dias** | | **400 pts** |

---

## 4. Impacto no Ranking

### 4.1 Cenário com Proposta Atual (sem mudanças)

| Posição | Usuário | Pontos | Dias |
|---------|---------|--------|------|
| 1º | Ana | 500 | 5 |
| 2º | Bruno | 480 | 5 |
| 3º | Carlos | 450 | 4 |

### 4.2 Cenário com Proposta Nova

| Posição | Usuário | Pontos | Dias | Diferença |
|---------|---------|--------|------|-----------|
| 1º | Ana | 620 | 5 | +120 pts |
| 2º | Bruno | 590 | 5 | +110 pts |
| 3º | Carlos | 510 | 4 | +60 pts |

**Observação:** A proposta aumenta a separação entre participantes ativos e ocasionais, incentivando engajamento consistente.

---

## 5. Valores Novos vs. Publicação

### 5.1 Regra Proposta

| Ação | Requisito |
|------|-----------|
| **Alterar CheckInPoints** | Aprovação do admin + log de auditoria |
| **Alterar MomentPoints** | Aprovação do admin + log de auditoria |
| **Alterar fórmulas de cálculo** | Aprovação do admin + deploy + migração |
| **Adicionar novas fontes** | Aprovação do admin + spec + implementação |

### 5.2 Segurança

- Valores novos **não são publicados sem aprovação explícita**
- Alterações ficam registradas no `manager_operations`
- Reversões de pontos exigem justificativa obrigatória

---

## 6. Próximos Passos

### 6.1 Para Aprovação

- [ ] Revisar proposta com equipe de produto
- [ ] Validar impacto em usuários existentes
- [ ] Definir valores de CheckInPoints e MomentPoints para novo evento
- [ ] Aprovar ou ajustar fórmulas

### 6.2 Para Implementação (após aprovação)

- [ ] Criar issues específicas para cada alteração
- [ ] Implementar caps diários no backend
- [ ] Adicionar multiplicador de participantes em runs
- [ ] Implementar sistema de streak
- [ ] Atualizar frontend para exibir novos valores
- [ ] Criar dashboard de auditoria de pontos

---

## 7. Conclusão

A proposta equilibra:
- **Incentivo à participação** (pontos por participação)
- **Recompensa à performance** (bônus por posição)
- **Engajamento consistente** (streak e caps)
- **Proteção contra farm** (limites diários)
- **Flexibilidade** (valores configuráveis)

**Recomendação:** Aprovar a proposta e criar issues de implementação por fase.

---

*Documento gerado pelo worker DNJ em 2026-09-07*