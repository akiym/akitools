use strict;
use warnings;
use Text::Xslate;
use Text::MultiMarkdown qw/markdown/;
use Data::Section::Simple qw/get_data_section/;
use Pod::Usage;

my $file = shift or pod2usage(0);

my $content = do {
    open my $fh, '<', $file or die $!;
    local $/; <$fh>
};

my @slides = map { markdown($_) } split /\n\n\n/, $content;
my ($title) = $slides[0] =~ m!<h1\s*[\w="]*>(.*?)</h1>!;

my $tx = Text::Xslate->new;
print $tx->render_string(get_data_section('slide.tx'), {
    title  => $title,
    slides => \@slides,
});

__END__

=head1 NAME

slide.pl - Slide generator written in Markdown

=head1 SYNOPSIS

    % slide.pl file

=cut

__DATA__

@@ slide.tx
<!doctype html>
<html>
<head>
    <meta charset="utf-8" />
    <title><: $title :></title>
    <link rel="stylesheet" href="../static/css/bootstrap.min.css" type="text/css" />
    <link rel="stylesheet" href="../static/css/slide.css" type="text/css" />
    <script type="text/javascript" src="../static/js/jquery-1.6.2.min.js"></script>
    <script type="text/javascript" src="../static/js/jquery.presentation.js"></script>
</head>
<body>
    <div id="slides">
    : for $slides -> $slide {
        <div class="slide">
<: $slide | mark_raw :>        </div>
    : }
    </div>
    <script>
$(function () {
    $('#slides').presentation();
});
    </script>
</body>
</html>
