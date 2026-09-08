;;; $DOOMDIR/config.el -*- lexical-binding: t; -*-

;; Place your private configuration here! Remember, you do not need to run 'doom
;; sync' after modifying this file!


;; Some functionality uses this to identify you, e.g. GPG configuration, email
;; clients, file templates and snippets. It is optional.
;; (setq user-full-name "John Doe"
;;       user-mail-address "john@doe.com")

;; Doom exposes five (optional) variables for controlling fonts in Doom:
;;
;; - `doom-font' -- the primary font to use
;; - `doom-variable-pitch-font' -- a non-monospace font (where applicable)
;; - `doom-big-font' -- used for `doom-big-font-mode'; use this for
;;   presentations or streaming.
;; - `doom-symbol-font' -- for symbols
;; - `doom-serif-font' -- for the `fixed-pitch-serif' face
;;
;; See 'C-h v doom-font' for documentation and more examples of what they
;; accept. For example:
;;
;;(setq doom-font (font-spec :family "Fira Code" :size 12 :weight 'semi-light)
;;      doom-variable-pitch-font (font-spec :family "Fira Sans" :size 13))
;;
;; If you or Emacs can't find your font, use 'M-x describe-font' to look them
;; up, `M-x eval-region' to execute elisp code, and 'M-x doom/reload-font' to
;; refresh your font settings. If Emacs still can't find your font, it likely
;; wasn't installed correctly. Font issues are rarely Doom issues!

;; There are two ways to load a theme. Both assume the theme is installed and
;; available. You can either set `doom-theme' or manually load a theme with the
;; `load-theme' function. This is the default:
(setq doom-theme 'doom-gruvbox)

;; This determines the style of line numbers in effect. If set to `nil', line
;; numbers are disabled. For relative line numbers, set this to `relative'.
(setq display-line-numbers-type t)

;; If you use `org' and don't want your org files in the default location below,
;; change `org-directory'. It must be set before org loads!
(setq org-directory "~/git/org/")


;; Whenever you reconfigure a package, make sure to wrap your config in an
;; `after!' block, otherwise Doom's defaults may override your settings. E.g.
;;
;;   (after! PACKAGE
;;     (setq x y))
;;
;; The exceptions to this rule:
;;
;;   - Setting file/directory variables (like `org-directory')
;;   - Setting variables which explicitly tell you to set them before their
;;     package is loaded (see 'C-h v VARIABLE' to look up their documentation).
;;   - Setting doom variables (which start with 'doom-' or '+').
;;
;; Here are some additional functions/macros that will help you configure Doom.
;;
;; - `load!' for loading external *.el files relative to this one
;; - `use-package!' for configuring packages
;; - `after!' for running code after a package has loaded
;; - `add-load-path!' for adding directories to the `load-path', relative to
;;   this file. Emacs searches the `load-path' when you load packages with
;;   `require' or `use-package'.
;; - `map!' for binding new keys
(map! :leader ";" #'comment-or-uncomment-region)

(map! :n "M-+" #'magit-status
      :i "M-+" #'magit-status)

(map! :n "C-q" #'pop-tag-mark
      :i "C-q" #'pop-tag-mark)

(map! :n "M-q" #'xref-find-definitions
      :i "M-q" #'xref-find-definitions)

(map! :leader "M-g" #'goto-line)

(map! :n "M-r" #'backward-word
      :i "M-r" #'backward-word)

(map! :n "M-l" #'drag-stuff-up
      :n "M-h" #'drag-stuff-down
      :i "M-l" #'drag-stuff-up
      :i "M-h" #'drag-stuff-down)

(map! :n "M-j" #'forward-paragraph
      :n "M-k" #'backward-paragraph
      :i "M-j" #'forward-paragraph
      :i "M-k" #'backward-paragraph)

(map! :i "C-i" #'evil-escape)

(map! :i "C-e" #'end-of-line
      :n "C-e" #'end-of-line)

(map! :n "M-d" #'delete-forward-char
      :i "M-d" #'delete-forward-char
      :n "M-s" #'delete-backward-char
      :i "M-s" #'delete-backward-char)

(map! :i "C-k" #'kill-line
      :n "C-k" #'kill-line)
(map! :i "C-y" #'yank
      :n "C-k" #'yank)
(map! :i "C-w" #'evil-delete
      :n "C-w" #'evil-delete)

(map! :n "C-x C-s" #'evil-write
      :i "C-x C-s" #'evil-write)

(setq scroll-margin 10)

(setq magit-repository-directories '("~/git/"))
(setq-default git-magit-status-fullscreen t)

(add-hook 'prog-mode-hook 'display-fill-column-indicator-mode)

(require 'lsp-mode)
;; (after! lsp-mode
;;   ;; https://github.com/emacs-lsp/lsp-mode/issues/3577#issuecomment-1709232622
;;   (delete 'lsp-go lsp-client-packages))

;; Enable LSP and set up auto-completion for Go
(after! lsp-mode
  ;; (setq lsp-enable-file-watchers nil)
  (setq lsp-file-watch-threshold 500)
  (map! :n "M-q" #'lsp-find-definition
        :i "M-q" #'lsp-find-definition
        :i "M--" #'lsp-find-references
        :n "M--" #'lsp-find-references
        :leader
        (:prefix "s"
         :desc "LSP Rename" "e" #'lsp-rename))
  (setq  lsp-go-analyses '((nilness . t)
                           (shadow . t)
                           (unusedparams . t)
                           (unusedwrite . t)
                           (useany . t)
                           (unusedvariable . t)))

  (add-hook 'go-mode-hook #'lsp)
  (add-hook 'go-mode-hook (lambda ()
                            (setq-local company-backends '(company-lsp))
                            (setq-local company-idle-delay 0.2))))


;; Ensure you have the gopls binary installed
(setq lsp-go-gopls-server-path (executable-find "gopls"))

(after! vterm
  (setq vterm-shell "zsh"))

(map! :n "M-1" #'winum-select-window-1
      :n "M-2" #'winum-select-window-2
      :n "M-3" #'winum-select-window-3
      :n "M-4" #'winum-select-window-4
      :n "M-5" #'winum-select-window-5
      :n "M-6" #'winum-select-window-6
      :n "M-7" #'winum-select-window-7
      :n "M-8" #'winum-select-window-8
      :n "M-9" #'winum-select-window-9)

;; To get information about any of these functions/macros, move the cursor over
;; the highlighted symbol at press 'K' (non-evil users must press 'C-c c k').
;; This will open documentation for it, including demos of how they are used.
;; Alternatively, use `C-h o' to look up a symbol (functions, variables, faces,
;; etc).
;;
;; You can also try 'gd' (or 'C-c c d') to jump to their definition and see how
;; they are implemented.

;; Set Emacs background transparency
;; Adjust transparency from 0 (fully transparent) to 100 (fully opaque)
(set-frame-parameter nil 'alpha-background 95)
