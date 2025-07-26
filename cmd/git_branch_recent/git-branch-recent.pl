#!/usr/bin/env perl
use strict;
use warnings;
use Term::ANSIColor;
use Time::Piece;

=pod
in your .zshrc:
function fzf-git-branch-activity-checkout () {
    local selected_branch_name=( $(git branch-recent |
      FZF_DEFAULT_OPTS="--height ${FZF_TMUX_HEIGHT:-40%} $FZF_DEFAULT_OPTS" fzf |
      perl -ne 'print $1 if /(\S+)$/')
    )
    local ret=$?
    if [ -n "$selected_branch_name" ]; then
        BUFFER="git checkout ${selected_branch_name}"
        zle accept-line
    fi
    zle redisplay
    typeset -f zle-line-init >/dev/null && zle zle-line-init
    return $ret
}
zle -N fzf-git-branch-activity-checkout
bindkey '^g' fzf-git-branch-activity-checkout
=cut

chomp(my $user = `git config --get user.name`);
my @skip_remote_branch = qw//;

my $lines = `git for-each-ref --count=100 --sort=-committerdate refs/ --format="%(authordate),%(authorname),%(refname)" --perl`;
for my $line (split /\n/, $lines) {
    my ($date, $author, $branch) = eval $line;
    next if $branch eq 'refs/stash';
    $date = localtime(
        Time::Piece->strptime($date, '%a %b %d %H:%M:%S %Y %z')->epoch
    )->strftime('[%Y-%m-%d %H:%M]');
    my $author_aligned = sprintf '%-16s', $author;
    $branch =~ s!^refs/(heads|remotes|tags)/?!!;
    my $is_remote = ($1 eq 'remotes');
    my $is_tag = ($1 eq 'tags');

    next if $is_remote && ($author eq $user || grep { $_ eq $branch } map { "origin/$_" } @skip_remote_branch);

    print join(' ',
        colored($date, 'blue'),
        colored($author_aligned, 'yellow'),
        $is_remote ? colored($branch, 'red') :
           $is_tag ? colored($branch, 'cyan') : $branch,
    );
    print "\n";
}
