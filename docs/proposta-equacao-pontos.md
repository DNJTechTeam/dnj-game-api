# Proposta de Pontuação — DNJ Game

**Issue:** KAN-13 (DNJ-011)  
**Status:** Proposta para aprovação  
**Escopo:** evento de um único dia

> Este documento é uma proposta de regra de negócio. Nenhum valor produtivo é alterado por esta tarefa.

## 1. Premissas do evento

- O evento acontece em apenas um dia.
- Não existem bônus por sequência de dias, streak ou progressão entre dias.
- Cada regra informa quando o participante pontua e se pode repetir.
- A pontuação individual é a base para o ranking de pessoas e para a soma dos grupos.
- O ranking individual distribui três prêmios: 1º, 2º e 3º lugar.
- O ranking de grupos soma os pontos individuais dos integrantes e distribui somente um prêmio: grupo vencedor.

## 2. Fontes de pontuação

### 2.1 Eventos radicais — Games

São atividades competitivas com quantidade variável de participantes. A quantidade de pessoas define a disputa, mas não altera os pontos dos colocados.

| Resultado | Pontos individuais |
|---|---:|
| 1º lugar | **50** |
| 2º lugar | **30** |
| 3º lugar | **20** |
| Participação | **10** |

Regras:

- O participante recebe os pontos correspondentes ao resultado obtido.
- Quem participa, mas não fica entre os três primeiros, recebe 10 pontos.
- A pontuação é individual e entra na soma do grupo.
- Cada participante pontua uma vez por participação naquela atividade.

### 2.2 Desafio Especial

O Desafio Especial não possui pontuação diferente para comunidade, grupo ou qualquer outra categoria de participante.

| Ação | Pontuação |
|---|---:|
| Escanear o QR Code e ter a participação validada | Pontos definidos para o desafio |

Regras:

- O participante pontua ao escanear o QR Code válido.
- A pontuação é individual.
- Não há classificação por colocação, multiplicador por grupo ou pontuação diferenciada por comunidade.
- Cada participante pode pontuar uma vez no mesmo Desafio Especial.

O valor do desafio deve ser configurado na atividade/QR Code. Esta proposta não publica um número novo para ele.

### 2.3 Desafio Momento

| Ação | Pontuação |
|---|---:|
| Tirar e enviar a foto do desafio, com validação aprovada | `MomentPoints` configurado |

Regras:

- O participante pontua quando a foto do Desafio Momento é registrada e aprovada conforme as regras de moderação.
- A pontuação é individual e entra na soma do grupo.
- Cada participante pontua uma vez por Desafio Momento.
- Momento livre, fora de um desafio pontuável, não gera pontos.

### 2.4 Check-in da programação

| Ação | Pontuação |
|---|---:|
| Check-in válido da programação/QR Code | `CheckInPoints` configurado |

Regras:

- O check-in válido gera pontos individuais.
- A idempotência impede que o mesmo QR Code seja usado repetidamente para farmar pontos.
- Como o evento tem um dia, não existe regra de streak ou bônus por dias consecutivos.

## 3. Ranking individual

O ranking individual soma todos os pontos válidos de cada participante durante o evento:

```text
total individual = games + participações + check-ins + Desafios Especiais + Desafios Momento
```

Premiação:

| Classificação | Prêmio |
|---|---|
| 1º lugar | Prêmio individual 1 |
| 2º lugar | Prêmio individual 2 |
| 3º lugar | Prêmio individual 3 |

Em caso de empate, a regra de desempate deve ser definida pelo produto antes da implementação da premiação. Esta proposta não inventa um critério de desempate.

## 4. Ranking de grupos

O ranking de grupos não cria pontos extras. Ele soma os pontos individuais dos integrantes:

```text
total do grupo = soma dos totais individuais dos integrantes
```

Regras:

- Todo ponto ganho por uma pessoa também compõe o total do grupo ao qual ela pertence.
- O grupo não recebe pontos adicionais por vencer uma atividade.
- O grupo vencedor é o grupo com maior soma ao final do evento.
- A premiação de grupo é única: somente o grupo vencedor ganha o prêmio.
- Se uma pessoa trocar de grupo, a regra de participação e o momento de consolidação precisam ser definidos antes da implementação. A proposta recomenda considerar o grupo vinculado no momento da pontuação.

## 5. Inventário resumido

| Fonte | Gera pontos? | Regra | Ranking individual? | Soma para grupo? |
|---|---|---|---|---|
| Game radical — 1º | Sim | 50 pontos | Sim | Sim |
| Game radical — 2º | Sim | 30 pontos | Sim | Sim |
| Game radical — 3º | Sim | 20 pontos | Sim | Sim |
| Game radical — participação | Sim | 10 pontos | Sim | Sim |
| Check-in da programação | Sim | `CheckInPoints` | Sim | Sim |
| Desafio Especial | Sim | QR validado, valor configurado | Sim | Sim |
| Desafio Momento | Sim | Foto validada, `MomentPoints` | Sim | Sim |
| Momento livre | Não | 0 pontos | Não | Não |
| Posição no ranking | Não | É consequência da soma | — | — |

## 6. Pontos que precisam de aprovação antes da implementação

- Valor padrão do Desafio Especial.
- Valores de `CheckInPoints` e `MomentPoints` para o evento.
- Critério de desempate do ranking individual.
- Critério de desempate do ranking de grupos.
- Regra para participante sem grupo.
- Regra para mudança de grupo durante o evento.
- Momento de fechamento e consolidação dos rankings.

## 7. Critérios para as próximas tarefas de implementação

- Os valores de games radicais devem ser 50, 30, 20 e 10.
- O Desafio Especial deve pontuar uma participação válida por pessoa, sem diferenciação de grupo ou comunidade.
- O Desafio Momento deve pontuar uma foto aprovada por pessoa em cada desafio.
- Pontos individuais devem alimentar simultaneamente o ranking individual e a soma do grupo.
- Deve existir somente uma premiação de grupo, para o grupo vencedor.
- Nenhum valor novo deve ser publicado sem aprovação explícita.

**Recomendação:** aprovar esta regra-base e abrir tarefas de implementação separadas para pontuação, ranking individual, ranking de grupos e premiação.
