<div align="center">

# agentguard 🛡️

**Un pare-feu policy-as-code déterministe pour les tool-calls d'un agent IA.**

Dire à un modèle *« ne supprime jamais la production »* dans son prompt ne tient
pas — lors de l'[incident Replit](https://www.theregister.com/2025/07/21/replit_saastr_ai_vibe_coding/),
un agent a supprimé une base de production malgré onze interdictions en
majuscules. agentguard ne demande pas au modèle de bien se comporter. Il
**intercepte l'action** — quel outil, sur quelle cible, dans quel contexte — et
applique une règle versionnée, comme un pare-feu applique une ACL : la même
entrée produit toujours la même décision.

Local-first et Kubernetes-native. C'est un **outil de confinement qui réduit le
rayon de souffle** d'un agent qui déraille ou qui a été détourné par injection de
prompt — un filet de sécurité, pas une garantie.

[English](README.md) · **Français**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8?logo=go&logoColor=white)](https://go.dev)

**[Pourquoi](#pourquoi) · [Démarrage](#démarrage) · [Comment ça marche](#comment-ça-marche) · [La policy](#la-policy) · [CI](#ci) · [Périmètre honnête](#périmètre-honnête) · [Roadmap](#roadmap)**

</div>

---

## Pourquoi

Un agent IA en production, c'est un problème **100% ops** : comment le sandboxer,
tracer ce qu'il fait, plafonner ce qu'il coûte ? L'industrie a appris à la dure
en 2025-26 qu'on ne résout pas ça dans le modèle :

- **Les règles au niveau du prompt échouent.** L'[OWASP LLM06](https://genai.owasp.org/llmrisk/llm06-excessive-agency/)
  est formel — *implémentez l'autorisation en aval, pas dans le LLM*. Un agent à
  qui on demande de ne pas faire une chose la fera quand même sous le bon prompt.
- **Le rayon de souffle est réel.** Un seul tool-call mal étiqueté ou injecté
  peut supprimer un namespace, lancer `terraform destroy` ou `rm -rf` un volume —
  et les outils obéissent sans broncher.

agentguard se place **entre l'agent et l'action** et applique une règle que
*vous* avez écrite et pouvez tester, indépendamment de ce que le modèle
« voulait ». La même idée qu'un pare-feu réseau depuis des décennies, appliquée
aux tool-calls d'un agent.

> C'est délibérément le frère d'[opsforge](https://github.com/Mrg77/opsforge) —
> les mêmes guards policy-as-code, généralisés d'*une commande que vous tapez* à
> *une action que votre agent tente*.

## Démarrage

```sh
# Compiler (Go 1.26+)
go build -o agentguard .

# Écrire une policy de départ fail-safe, lisible, éditable et committable
./agentguard init

# Simuler une action d'agent — le guard l'autoriserait-il ?
./agentguard policy test \
  --tool shell --target "kubectl delete namespace payments" --context prod
```

```text
────────────────────────────────────────────────────────────
  ❯ agentguard policy test
  would this action be allowed?
────────────────────────────────────────────────────────────

  tool    shell
  target  kubectl delete namespace payments
  context prod

  ✗ DENY  Deleting a production namespace is forbidden by policy.  [no deleting prod namespaces]
```

La même action hors prod, ou en lecture seule, passe directement :

```sh
./agentguard policy test --tool kubectl --target "kubectl get pods" --context prod
#   ✓ ALLOW  [allow read-only kubectl anywhere]
```

## Comment ça marche

Une **action** = trois chaînes que le guard n'interprète jamais sémantiquement :

| Champ | Ce que c'est | Exemple |
|---|---|---|
| `tool` | l'outil/fonction que l'agent invoque | `shell`, `kubectl`, `http.post`, un nom d'outil MCP |
| `target` | ce sur quoi l'outil agit | `kubectl delete namespace payments` |
| `context` | l'environnement actif | `prod`, `gke_prod-eu`, `staging` |

Le moteur essaie les règles **de haut en bas, la première qui matche gagne**, et
renvoie l'une des trois décisions :

- **`allow`** — l'action passe.
- **`ask`** — elle est retenue jusqu'à validation humaine (le garde-fou de
  confirmation).
- **`deny`** — elle est bloquée net.

Si aucune règle ne matche, le `default` de la policy s'applique (fail-safe : le
défaut livré est `ask`). Le moteur est **pur** — pas d'I/O, pas d'horloge, pas de
modèle — donc la même entrée donne toujours la même décision, et chaque règle est
testable unitairement.

## La policy

`agentguard init` écrit un `agentguard.yaml` commenté et **fail-safe**. Les règles
sont des regexes insensibles à la casse ; un champ vide matche tout ; une chaîne
simple se comporte comme une recherche de sous-chaîne :

```yaml
default: ask            # aucune règle matchée → un humain décide

rules:
  - name: no deleting prod namespaces
    tool: "kubectl|shell"
    target: "delete\\s+(namespace|ns)\\b"
    context: "prod"
    action: deny
    message: "Deleting a production namespace is forbidden by policy."

  - name: confirm destructive kubectl on prod
    target: "kubectl\\s+(delete|drain|cordon|scale|rollout\\s+restart)"
    context: "prod"
    action: ask

  - name: allow read-only kubectl anywhere
    target: "kubectl\\s+(get|describe|logs|top)\\b"
    action: allow
```

Comme c'est un simple fichier, la policy vit dans votre dépôt, se relit en PR et
se teste en CI — au lieu d'être bricolée à la main sur chaque machine. `rm -rf`
est matché sur la cible seule (pas de contrainte `tool`) pour ne pas pouvoir être
contourné en mal-étiquetant l'appel — exactement le genre de durcissement pour
lequel toute l'approche existe.

## CI

`policy test` encode sa décision dans le code de sortie : une action refusée
**fait échouer le job** — vous pouvez vérifier que vos garde-fous tiennent à
chaque commit :

```sh
# échoue (exit 2) si la policy laisserait un agent détruire la prod
agentguard policy test --target "terraform destroy" --context prod
```

```yaml
# .github/workflows/guardrails.yml
- run: |
    ! agentguard policy test --target "kubectl delete namespace payments" --context prod
    ! agentguard policy test --target "terraform destroy" --context prod
```

Ajoutez `--json` à n'importe quelle commande pour une sortie lisible par machine.

## Périmètre honnête

La chose la plus importante que cet outil dit sur lui-même :

- **Il confine, il ne garantit pas.** agentguard réduit le rayon de souffle d'un
  agent qui déraille. Il **ne peut pas** stopper l'injection de prompt — il n'y a
  [pas de défense universelle](https://simonwillison.net/2025/Apr/11/camel/) — et
  il ne prétend pas détecter l'intention.
- **Il applique sur l'action, pas sur le modèle.** Toute la valeur est
  précisément qu'il ignore ce que le modèle « voulait » et matche ce qu'il *a
  tenté de faire*.
- **Un guard auquel on ne peut pas se fier est pire que rien.** Une policy
  invalide est une erreur bruyante, jamais un raté silencieux. Le défaut est
  fail-safe (`ask`).

Cette honnêteté est une fonctionnalité. Un outil qui sur-promettrait la « sécurité
IA » serait le moins digne de confiance.

## Roadmap

Le MVP à quatre commandes est complet — moteur, démo locale, enforcement runtime,
scan supply-chain — chacune validée de bout en bout sur une vraie machine :

- [x] Le moteur de policy déterministe (`policy test`, `init`) — allow/ask/deny
- [x] `agentguard up` / `down` — un cluster kind jetable + un modèle Ollama local
      en une commande, sur un laptop, en CPU seul (kubeconfig isolé, jamais le
      vôtre)
- [x] `agentguard proxy` — un serveur MCP que l'agent interroge avant d'agir ;
      applique la policy sur les tool-calls en direct et enregistre chaque
      décision dans un journal d'audit (`agentguard log`) avec un compte de tokens
- [x] `agentguard scan` — un audit supply-chain en lecture seule des serveurs MCP
      auxquels un agent se connecte (code distant non épinglé, secrets en clair, HTTP)

---

<div align="center">
Construit par <a href="https://github.com/Mrg77">Mrg77</a> · frère d'<a href="https://github.com/Mrg77/opsforge">opsforge</a> · MIT
</div>
