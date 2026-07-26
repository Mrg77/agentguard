<div align="center">

# agentguard 🛡️

**Un garde-fou qui empêche un agent IA de faire une bêtise irréversible sur votre infra.**

[English](README.md) · **Français**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Made with Go](https://img.shields.io/badge/made%20with-Go-00ADD8?logo=go&logoColor=white)](https://go.dev)

![démo agentguard](demo/demo.gif)

</div>

---

## Le problème, en une histoire vraie

En juillet 2025, un développeur laisse un agent IA travailler sur son projet. Dans ses
instructions, il avait écrit — en majuscules, onze fois — de **ne jamais toucher à la
base de production**. L'agent l'a supprimée quand même. ([le récit complet, en anglais](https://www.theregister.com/2025/07/21/replit_saastr_ai_vibe_coding/))

C'est le cœur du sujet. Un agent IA moderne ne se contente plus de répondre : il **agit**.
Il lance des commandes, appelle des outils, modifie des fichiers. Et on lui donne de plus
en plus les clés de l'infrastructure. Le problème, c'est que **lui interdire quelque chose
dans son prompt ne suffit pas** : sous la bonne formulation, ou après une injection
malveillante, il le fera quand même. Ce n'est pas une opinion, c'est un constat que la
communauté sécurité a formalisé (voir l'[OWASP LLM06](https://genai.owasp.org/llmrisk/llm06-excessive-agency/),
qui dit noir sur blanc : *l'autorisation doit être vérifiée en dehors du modèle, pas à
l'intérieur*).

**agentguard part d'un principe simple : on n'essaie pas de convaincre l'IA de bien se
comporter. On l'en empêche techniquement.**

L'idée est vieille comme le réseau : un **pare-feu**. Un pare-feu réseau ne demande pas
gentiment aux paquets de ne pas passer — il les bloque. agentguard fait pareil, mais pour
les actions d'un agent IA. Avant chaque action, il vérifie une **règle que vous avez
écrite** et décide : je laisse passer, je demande confirmation, ou je bloque. La règle est
la même à chaque fois, quoi que le modèle « ait voulu dire ».

> **Une chose à dire tout de suite, par honnêteté :** agentguard ne rend pas votre agent
> IA « sûr ». Il *réduit les dégâts possibles*. Il ne peut pas empêcher une injection de
> prompt (personne ne sait faire ça de façon fiable), et il ne devine pas les intentions.
> Ce qu'il fait, et fait bien : empêcher une action dangereuse d'atteindre votre
> infrastructure. Un filet de sécurité, pas une garantie — et c'est déjà énorme.

*(agentguard est le petit frère d'[opsforge](https://github.com/Mrg77/opsforge) : le même
principe de garde-fous, appliqué là à une commande que **vous** tapez, ici à une action
que **votre agent** tente.)*

---

## Essayer en une minute

Il vous faut juste [Go](https://go.dev) (version 1.26 ou plus récente).

```sh
git clone https://github.com/Mrg77/agentguard && cd agentguard
go build -o agentguard .

# On génère une politique de départ (des règles prêtes à l'emploi)
./agentguard init

# On simule une action : « l'agent voudrait supprimer un namespace de prod » — OK ou pas ?
./agentguard policy test \
  --tool shell --target "kubectl delete namespace payments" --context prod
```

Ce que vous voyez :

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

Bloqué. La même action, mais en lecture seule ou hors production, passe sans problème :

```sh
./agentguard policy test --tool kubectl --target "kubectl get pods" --context prod
#   ✓ ALLOW
```

C'est tout le principe : **la décision dépend de ce que l'action fait vraiment et d'où
elle s'exécute, jamais de ce que le modèle prétend vouloir.**

---

## Comment ça marche, concrètement

Pour agentguard, une **action** se résume à trois informations — trois bouts de texte
qu'il ne cherche jamais à « comprendre », juste à faire correspondre à des règles :

| L'info | Ce que c'est | Exemple |
|---|---|---|
| **l'outil** (`tool`) | quel outil ou fonction l'agent appelle | `shell`, `kubectl`, `http.post`… |
| **la cible** (`target`) | ce sur quoi l'action porte | `kubectl delete namespace payments` |
| **le contexte** (`context`) | l'environnement où ça s'exécute | `prod`, `staging`, `gke_prod-eu`… |

À partir de ça, le moteur regarde vos règles **de haut en bas et s'arrête à la première
qui correspond**. Il rend alors l'une de trois décisions :

- **`allow`** — l'action passe, rien à signaler.
- **`ask`** — on met en pause et on demande à un humain de trancher.
- **`deny`** — on bloque, point.

Si aucune règle ne correspond, c'est la valeur par défaut qui s'applique. Et par défaut,
agentguard choisit la prudence (`ask` : un humain décide) plutôt que de laisser passer.

Un détail qui a son importance : ce moteur est **complètement déterministe**. Pas d'appel
réseau, pas d'IA, pas de hasard. La même action donne *toujours* la même réponse. C'est ce
qui le rend testable, prévisible, et digne de confiance — l'inverse d'un système qui
« devine ».

---

## Les règles : un simple fichier que vous versionnez

`agentguard init` vous écrit un fichier `agentguard.yaml` commenté et prudent par défaut.
Vous le lisez, vous l'adaptez, vous le committez dans votre dépôt — au même endroit que le
code qu'il protège. Voici à quoi ressemble une règle :

```yaml
default: ask            # si aucune règle ne matche → un humain décide

rules:
  - name: pas de suppression de namespace en prod
    tool: "kubectl|shell"
    target: "delete\\s+(namespace|ns)\\b"
    context: "prod"
    action: deny
    message: "Supprimer un namespace de production est interdit par la politique."

  - name: demander avant un kubectl destructeur en prod
    target: "kubectl\\s+(delete|drain|cordon|scale|rollout\\s+restart)"
    context: "prod"
    action: ask

  - name: laisser passer la lecture seule partout
    target: "kubectl\\s+(get|describe|logs|top)\\b"
    action: allow
```

Les motifs de correspondance sont des expressions régulières (insensibles à la casse), mais
pas besoin d'être un expert : un simple mot se comporte comme une recherche « contient ce
texte ». Un champ laissé vide veut dire « n'importe quoi ».

Comme c'est un fichier, il vit dans votre dépôt, se relit dans une *pull request*, et se
teste automatiquement (voir la section CI plus bas) — au lieu d'être bricolé différemment
sur chaque machine.

> Petit exemple de finesse : la règle qui bloque `rm -rf` ne précise pas d'outil.
> Pourquoi ? Parce que `rm -rf` est dangereux quel que soit l'outil qui le lance. Si on
> avait exigé « outil = shell », il aurait suffi à l'agent de mal étiqueter son appel pour
> passer à travers. C'est exactement le genre de trou que ce genre d'outil doit éviter.

---

## Deux façons de brancher agentguard : le conseil, ou le vrai barrage

C'est **le** point à bien comprendre, parce que les deux modes ne protègent pas de la même
façon.

D'abord, un mot sur **MCP** (*Model Context Protocol*). C'est le standard ouvert, apparu en
2024-2025, qui permet à un agent IA de se brancher sur des outils. Claude Code, Cursor,
VS Code, Windsurf, Zed… tous le parlent. agentguard parle MCP lui aussi, ce qui veut dire
une chose pratique : **il marche avec n'importe quel éditeur/agent compatible MCP, vous
n'êtes lié à aucun**.

Maintenant, les deux modes :

| Mode | Comment il se place | L'agent peut-il l'ignorer ? |
|---|---|---|
| **`agentguard proxy`** *(mode conseil)* | il propose un outil `guard` que l'agent **choisit** d'appeler avant d'agir | **Oui.** Un agent mal élevé peut l'ignorer et appeler l'outil directement. Utile surtout pour la traçabilité. |
| **`agentguard interpose`** *(le vrai pare-feu)* | il se place **sur le chemin**, entre l'agent et l'outil réel | **Non.** L'agent n'a plus accès qu'à agentguard. Chaque appel *doit* passer par la règle. |

Le mode `interpose` est celui qui mérite le mot « pare-feu ». Voici l'idée :

```text
interpose :  l'agent ──► agentguard ──► le vrai serveur d'outils (kubectl, fichiers…)
                              │ autorisé → on relaie à l'outil, la réponse revient normalement
                              │ refusé   → l'outil n'est JAMAIS appelé ; l'agent reçoit un refus
```

En clair : vous branchez **agentguard** sur votre agent, *pas* l'outil réel. L'agent croit
parler à l'outil, mais tout passe d'abord par le garde-fou. Il ne peut pas le contourner,
tout simplement parce qu'il n'a plus l'outil sous la main — seulement agentguard.

```sh
# Mettre agentguard devant un vrai serveur d'outils (ici, l'accès aux fichiers).
# On enregistre AGENTGUARD auprès de l'agent, pas le serveur d'origine.
claude mcp add fs -- agentguard interpose --context prod -- \
  npx -y @modelcontextprotocol/server-filesystem /data
```

`interpose` recopie à l'identique les outils du serveur d'origine (mêmes noms, mêmes
paramètres) : l'agent voit exactement le même serveur, il l'atteint juste à travers le
garde-fou. Et dans les deux modes, chaque décision est enregistrée dans un journal
(`agentguard log`).

---

## Le journal : qu'a tenté mon agent ?

Chaque fois qu'agentguard prend une décision, il l'écrit dans un journal local. `agentguard
log` vous le rejoue :

```sh
agentguard log --prod            # seulement les contextes de production
agentguard log --decision deny   # seulement ce qui a été bloqué
agentguard log --since 7d        # la dernière semaine
```

C'est votre trace : *qu'a essayé de faire mon agent contre la prod cette semaine, et le
garde-fou l'a-t-il laissé passer ?* Une question à laquelle la conversation brute avec le
modèle ne répond pas. Le fichier est en JSON ligne par ligne, à un emplacement connu, donc
facile à expédier vers un outil d'équipe (Loki, un SIEM…) si vous voulez la vue « flotte ».

---

## Vérifier d'où vient le code des serveurs d'outils

Petit bonus dans le même esprit sécurité. Avant de laisser un agent charger un serveur
d'outils MCP, autant savoir d'où vient son code. `agentguard scan` lit la config MCP de
votre agent (le fichier de Claude Code, Cursor…) et signale les drapeaux rouges — **sans
jamais lancer aucun serveur** :

```sh
agentguard scan ~/.cursor/mcp.json
```

Il repère par exemple : du code téléchargé et exécuté à chaque démarrage sans version figée
(`npx` sans numéro de version), un secret écrit en clair dans la config, ou un serveur
contacté en HTTP non chiffré. C'est en lecture seule et hors-ligne — il regarde juste la
commande de lancement déclarée.

---

## La démo en local : un vrai cluster, un vrai modèle

Pour voir le garde-fou à l'œuvre dans un cadre réaliste, agentguard peut monter tout un
terrain de jeu jetable sur votre machine — un vrai cluster Kubernetes ([kind](https://kind.sigs.k8s.io))
avec un petit modèle qui tourne sur CPU ([Ollama](https://ollama.com)), le tout en une
commande. Pas besoin de cloud ni de carte graphique.

```sh
agentguard up        # monte le cluster + le modèle
agentguard down      # supprime tout
```

**Rassurant à savoir :** agentguard crée son **propre** cluster avec sa **propre**
configuration Kubernetes, dans un coin isolé. Il ne touche jamais à votre `~/.kube/config` :
impossible qu'il se connecte par erreur à un vrai cluster de production.

---

## L'intégrer dans votre CI

`agentguard policy test` renvoie un code de sortie qui encode la décision (2 = refusé).
Autrement dit, vous pouvez faire **échouer un build** si votre politique laisserait passer
quelque chose de dangereux — une façon de vérifier, à chaque commit, que vos garde-fous
tiennent toujours :

```yaml
# .github/workflows/guardrails.yml
- run: |
    # ces lignes échouent (et cassent le build) si la politique ne bloque PLUS ces actions
    ! agentguard policy test --target "kubectl delete namespace payments" --context prod
    ! agentguard policy test --target "terraform destroy" --context prod
```

Ajoutez `--json` à n'importe quelle commande pour une sortie lisible par une machine.

---

## Ce que ça coûte (spoiler : rien en tokens)

Question légitime quand on parle d'IA : **est-ce que ça consomme des tokens ?**

**Non. agentguard ne fait aucun appel à un modèle de langage, donc il ne consomme aucun
token pour lui-même.** Sa décision, c'est de la comparaison de texte pure — rapide, locale,
gratuite. C'est un choix assumé, et l'exact opposé des approches « on demande à un LLM de
juger » qui, elles, factureraient un appel modèle à chaque décision.

Le seul coût côté agent :

- **En mode `interpose`** : négligeable. Les actions autorisées sont relayées telles quelles.
  La seule dépense, c'est un court message de refus (quelques dizaines de tokens) que l'agent
  lit quand une action est bloquée — bien moins cher que l'action dangereuse qu'on vient
  d'éviter.
- **En mode `proxy`** : un petit surcoût, l'agent posant une question `guard(...)` avant
  chaque action gardée.

Le `~N tok` que vous voyez dans `agentguard log` est juste une **estimation de la taille de
l'action** (pour le suivi de coûts), pas une facture, et surtout pas des tokens
qu'agentguard aurait dépensés.

---

## Le périmètre, en toute franchise

Le plus important à retenir sur ce que cet outil est — et n'est pas :

- **Il contient, il ne garantit pas.** Il réduit les dégâts possibles d'un agent qui
  déraille. Il **ne peut pas** stopper une injection de prompt — [il n'existe pas de défense
  universelle](https://simonwillison.net/2025/Apr/11/camel/) — et il ne prétend pas lire les
  intentions.
- **Il agit sur l'action, pas sur le modèle.** Toute sa valeur est là : il se moque de ce que
  le modèle « voulait » et regarde ce qu'il *a essayé de faire*.
- **« Pare-feu » veut dire `interpose`.** Seul ce mode est incontournable. Le mode `proxy`
  est un conseil : il suppose que l'agent joue le jeu.
- **Un garde-fou en qui on ne peut pas avoir confiance est pire que rien.** Une politique
  invalide provoque une erreur bien visible, jamais un silence trompeur. Et le défaut est
  prudent.

Cette franchise est une fonctionnalité, pas un aveu de faiblesse. Un outil qui sur-promettrait
la « sécurité de l'IA » serait justement celui en qui il ne faudrait pas avoir confiance.

---

## Où en est le projet

Le cœur est complet, et chaque brique a été testée de bout en bout sur une vraie machine :

- [x] **Le moteur de décision** (`policy test`, `init`) — allow / ask / deny, déterministe
- [x] **`agentguard up` / `down`** — cluster jetable + modèle local en une commande (sur un
      laptop, sans GPU, avec sa propre config isolée)
- [x] **`agentguard proxy`** — le mode conseil, avec le journal d'audit
- [x] **`agentguard interpose`** — **le vrai pare-feu** : garde-fou incontournable devant un
      serveur d'outils réel
- [x] **`agentguard scan`** — l'audit d'où vient le code des serveurs MCP

Envie d'essayer chaque niveau vous-même ? Le guide [TESTING.md](TESTING.md) vous prend par la
main, du plus simple (juste Go) au plus complet (un vrai cluster).

---

<div align="center">
Construit par <a href="https://github.com/Mrg77">Mrg77</a> · le frère d'<a href="https://github.com/Mrg77/opsforge">opsforge</a> · MIT
</div>
