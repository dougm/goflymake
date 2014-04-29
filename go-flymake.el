(require 'go-mode)
(require 'flymake)

(defgroup goflymake nil
  "Support for Flymake in Go via goflymake"
  :group 'go)

(defcustom goflymake-debug nil
  "Enable failure debugging mode in goflymake."
  :type 'boolean
  :group 'goflymake)

;; flymake.el's flymake-create-temp-inplace appends a
;; '_flymake' suffix to file-name.
;; this version prepends a 'flymake_' prefix, since the go tools look
;; at suffix for '_test', '_$goos', etc.
(defun goflymake-create-temp-inplace (file-name prefix)
  (unless (stringp file-name)
    (error "Invalid file-name"))
  (or prefix
      (setq prefix "flymake"))
  (let* ((temp-name (concat (file-name-directory file-name)
			      prefix "_" (file-name-nondirectory file-name))))
    (flymake-log 3 "create-temp-inplace: file=%s temp=%s" file-name temp-name)
    temp-name))

(defun goflymake-init ()
  (let* ((temp-file (flymake-init-create-temp-buffer-copy
                     'goflymake-create-temp-inplace))
         (local-file (file-relative-name
                      temp-file
                      (file-name-directory buffer-file-name))))
    (list "goflymake"
          (list (if goflymake-debug "-debug=true" "-debug=false")
                temp-file))))

(push '(".+\\.go$" goflymake-init) flymake-allowed-file-name-masks)

(add-hook 'go-mode-hook 'flymake-mode)

(provide 'go-flymake)
